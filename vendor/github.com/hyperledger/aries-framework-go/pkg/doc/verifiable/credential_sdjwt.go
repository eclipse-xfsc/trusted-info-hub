/*
Copyright Avast Software. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package verifiable

import (
	"crypto"
	"encoding/json"
	"fmt"

	"github.com/hyperledger/aries-framework-go/pkg/doc/jose"
	"github.com/hyperledger/aries-framework-go/pkg/doc/sdjwt/common"
	"github.com/hyperledger/aries-framework-go/pkg/doc/sdjwt/holder"
	"github.com/hyperledger/aries-framework-go/pkg/doc/sdjwt/issuer"
	json2 "github.com/hyperledger/aries-framework-go/pkg/doc/util/json"
)

type marshalDisclosureOpts struct {
	includeAllDisclosures bool
	discloseIfAvailable   []string
	discloseRequired      []string
	holderBinding         *holder.BindingInfo
	signer                jose.Signer
	signingKeyID          string
}

// MarshalDisclosureOption provides an option for Credential.MarshalWithDisclosure.
type MarshalDisclosureOption func(opts *marshalDisclosureOpts)

// TODO: should DiscloseGiven(IfAvailable|Required) have path semantics for disclosure?

// DiscloseGivenIfAvailable sets that the disclosures with the given claim names will be disclosed by
// Credential.MarshalWithDisclosure.
//
// If any name provided does not have a matching disclosure, Credential.MarshalWithDisclosure will skip the name.
//
// Will result in an error if this option is provided alongside DiscloseAll.
func DiscloseGivenIfAvailable(disclosureNames []string) MarshalDisclosureOption {
	return func(opts *marshalDisclosureOpts) {
		opts.discloseIfAvailable = disclosureNames
	}
}

// DiscloseGivenRequired sets that the disclosures with the given claim names will be disclosed by
// Credential.MarshalWithDisclosure.
//
// If any name provided does not have a matching disclosure, Credential.MarshalWithDisclosure will return an error.
//
// Will result in an error if this option is provided alongside DiscloseAll.
func DiscloseGivenRequired(disclosureNames []string) MarshalDisclosureOption {
	return func(opts *marshalDisclosureOpts) {
		opts.discloseRequired = disclosureNames
	}
}

// DiscloseAll sets that all disclosures in the given Credential will be disclosed by Credential.MarshalWithDisclosure.
//
// Will result in an error if this option is provided alongside DiscloseGivenIfAvailable or DiscloseGivenRequired.
func DiscloseAll() MarshalDisclosureOption {
	return func(opts *marshalDisclosureOpts) {
		opts.includeAllDisclosures = true
	}
}

// DisclosureHolderBinding option configures Credential.MarshalWithDisclosure to include a holder binding.
func DisclosureHolderBinding(binding *holder.BindingInfo) MarshalDisclosureOption {
	return func(opts *marshalDisclosureOpts) {
		opts.holderBinding = binding
	}
}

// DisclosureSigner option provides Credential.MarshalWithDisclosure with a signer that will be used to create an SD-JWT
// if the given Credential wasn't already parsed from SD-JWT.
func DisclosureSigner(signer jose.Signer, signingKeyID string) MarshalDisclosureOption {
	return func(opts *marshalDisclosureOpts) {
		opts.signer = signer
		opts.signingKeyID = signingKeyID
	}
}

// MarshalWithDisclosure marshals a SD-JWT credential in combined format for presentation, including precisely
// the disclosures indicated by provided options, and optionally a holder binding if given the requisite option.
func (vc *Credential) MarshalWithDisclosure(opts ...MarshalDisclosureOption) (string, error) {
	options := &marshalDisclosureOpts{}

	for _, opt := range opts {
		opt(options)
	}

	if options.includeAllDisclosures && (len(options.discloseIfAvailable) > 0 || len(options.discloseRequired) > 0) {
		return "", fmt.Errorf("incompatible options provided")
	}

	if vc.JWT != "" && vc.SDJWTHashAlg != "" {
		return filterSDJWTVC(vc, options)
	}

	if options.signer == nil {
		return "", fmt.Errorf("credential needs signer to create SD-JWT")
	}

	return createSDJWTPresentation(vc, options)
}

func filterSDJWTVC(vc *Credential, options *marshalDisclosureOpts) (string, error) {
	disclosureCodes, err := filteredDisclosureCodes(vc.SDJWTDisclosures, options)
	if err != nil {
		return "", err
	}

	cf := common.CombinedFormatForPresentation{
		SDJWT:         vc.JWT,
		Disclosures:   disclosureCodes,
		HolderBinding: vc.SDHolderBinding,
	}

	if options.holderBinding != nil {
		cf.HolderBinding, err = holder.CreateHolderBinding(options.holderBinding)
		if err != nil {
			return "", fmt.Errorf("failed to create holder binding: %w", err)
		}
	}

	return cf.Serialize(), nil
}

func createSDJWTPresentation(vc *Credential, options *marshalDisclosureOpts) (string, error) {
	issued, err := makeSDJWT(vc, options.signer, options.signingKeyID)
	if err != nil {
		return "", fmt.Errorf("creating SD-JWT from Credential: %w", err)
	}

	disclosureClaims, err := common.GetDisclosureClaims(issued.Disclosures)
	if err != nil {
		return "", fmt.Errorf("parsing disclosure claims from vc sdjwt: %w", err)
	}

	disclosureCodes, err := filteredDisclosureCodes(disclosureClaims, options)
	if err != nil {
		return "", err
	}

	var presOpts []holder.Option

	if options.holderBinding != nil {
		presOpts = append(presOpts, holder.WithHolderBinding(options.holderBinding))
	}

	issuedSerialized, err := issued.Serialize(false)
	if err != nil {
		return "", fmt.Errorf("serializing SD-JWT for presentation: %w", err)
	}

	combinedSDJWT, err := holder.CreatePresentation(issuedSerialized, disclosureCodes, presOpts...)
	if err != nil {
		return "", fmt.Errorf("create SD-JWT presentation: %w", err)
	}

	return combinedSDJWT, nil
}

func filteredDisclosureCodes(
	availableDisclosures []*common.DisclosureClaim,
	options *marshalDisclosureOpts,
) ([]string, error) {
	var (
		useDisclosures  []*common.DisclosureClaim
		err             error
		disclosureCodes []string
	)

	if options.includeAllDisclosures {
		useDisclosures = availableDisclosures
	} else {
		useDisclosures, err = filterDisclosures(availableDisclosures,
			options.discloseIfAvailable, options.discloseRequired)
		if err != nil {
			return nil, err
		}
	}

	for _, disclosure := range useDisclosures {
		disclosureCodes = append(disclosureCodes, disclosure.Disclosure)
	}

	return disclosureCodes, nil
}

func filterDisclosures(
	disclosures []*common.DisclosureClaim,
	ifAvailable, required []string,
) ([]*common.DisclosureClaim, error) {
	ifAvailMap := map[string]*common.DisclosureClaim{}
	reqMap := map[string]*common.DisclosureClaim{}

	for _, name := range ifAvailable {
		ifAvailMap[name] = nil
	}

	for _, name := range required {
		reqMap[name] = nil

		delete(ifAvailMap, name) // avoid listing a disclosure twice, if it's in both lists
	}

	for _, disclosure := range disclosures {
		if _, ok := ifAvailMap[disclosure.Name]; ok {
			ifAvailMap[disclosure.Name] = disclosure
		}

		if _, ok := reqMap[disclosure.Name]; ok {
			reqMap[disclosure.Name] = disclosure
		}
	}

	var out []*common.DisclosureClaim

	for _, claim := range ifAvailMap {
		if claim != nil {
			out = append(out, claim)
		}
	}

	for _, claim := range reqMap {
		if claim == nil {
			return nil, fmt.Errorf("disclosure list missing required claim")
		}

		out = append(out, claim)
	}

	return out, nil
}

type makeSDJWTOpts struct {
	hashAlg crypto.Hash
}

// MakeSDJWTOption provides an option for creating an SD-JWT from a VC.
type MakeSDJWTOption func(opts *makeSDJWTOpts)

// MakeSDJWTWithHash sets the hash to use for an SD-JWT VC.
func MakeSDJWTWithHash(hash crypto.Hash) MakeSDJWTOption {
	return func(opts *makeSDJWTOpts) {
		opts.hashAlg = hash
	}
}

// MakeSDJWT creates an SD-JWT in combined format for issuance, with all fields in credentialSubject converted
// recursively into selectively-disclosable SD-JWT claims.
func (vc *Credential) MakeSDJWT(signer jose.Signer, signingKeyID string, options ...MakeSDJWTOption) (string, error) {
	sdjwt, err := makeSDJWT(vc, signer, signingKeyID, options...)
	if err != nil {
		return "", err
	}

	sdjwtSerialized, err := sdjwt.Serialize(false)
	if err != nil {
		return "", fmt.Errorf("serializing SD-JWT: %w", err)
	}

	return sdjwtSerialized, nil
}

func makeSDJWT(vc *Credential, signer jose.Signer, signingKeyID string, options ...MakeSDJWTOption,
) (*issuer.SelectiveDisclosureJWT, error) {
	opts := &makeSDJWTOpts{}

	for _, option := range options {
		option(opts)
	}

	claims, err := vc.JWTClaims(false)
	if err != nil {
		return nil, fmt.Errorf("constructing VC JWT claims: %w", err)
	}

	claimBytes, err := json.Marshal(claims)
	if err != nil {
		return nil, err
	}

	claimMap := map[string]interface{}{}

	err = json.Unmarshal(claimBytes, &claimMap)
	if err != nil {
		return nil, err
	}

	headers := map[string]interface{}{
		jose.HeaderKeyID: signingKeyID,
	}

	issuerOptions := []issuer.NewOpt{
		issuer.WithStructuredClaims(true),
		issuer.WithNonSelectivelyDisclosableClaims([]string{"id"}),
	}

	if opts.hashAlg != 0 {
		issuerOptions = append(issuerOptions, issuer.WithHashAlgorithm(opts.hashAlg))
	}

	sdjwt, err := issuer.NewFromVC(claimMap, headers, signer, issuerOptions...)
	if err != nil {
		return nil, fmt.Errorf("creating SD-JWT from VC: %w", err)
	}

	return sdjwt, nil
}

type displayCredOpts struct {
	displayAll   bool
	displayGiven []string
}

// DisplayCredentialOption provides an option for Credential.CreateDisplayCredential.
type DisplayCredentialOption func(opts *displayCredOpts)

// DisplayAllDisclosures sets that Credential.CreateDisplayCredential will include all disclosures in the generated
// credential.
func DisplayAllDisclosures() DisplayCredentialOption {
	return func(opts *displayCredOpts) {
		opts.displayAll = true
	}
}

// DisplayGivenDisclosures sets that Credential.CreateDisplayCredential will include only the given disclosures in the
// generated credential.
func DisplayGivenDisclosures(given []string) DisplayCredentialOption {
	return func(opts *displayCredOpts) {
		opts.displayGiven = append(opts.displayGiven, given...)
	}
}

// CreateDisplayCredential creates, for SD-JWT credentials, a Credential whose selective-disclosure subject fields
// are replaced with the disclosure data.
//
// Options may be provided to filter the disclosures that will be included in the display credential. If a disclosure is
// not included, the associated claim will not be present in the returned credential.
//
// If the calling Credential is not an SD-JWT credential, this method returns the credential itself.
func (vc *Credential) CreateDisplayCredential( // nolint:funlen,gocyclo
	opts ...DisplayCredentialOption,
) (*Credential, error) {
	options := &displayCredOpts{}

	for _, opt := range opts {
		opt(options)
	}

	if options.displayAll && len(options.displayGiven) > 0 {
		return nil, fmt.Errorf("incompatible options provided")
	}

	if vc.SDJWTHashAlg == "" || vc.JWT == "" {
		return vc, nil
	}

	credClaims, err := unmarshalJWSClaims(vc.JWT, false, nil)
	if err != nil {
		return nil, fmt.Errorf("unmarshal VC JWT claims: %w", err)
	}

	credClaims.refineFromJWTClaims()

	useDisclosures := filterDisclosureList(vc.SDJWTDisclosures, options)

	newVCObj, err := common.GetDisclosedClaims(useDisclosures, credClaims.VC)
	if err != nil {
		return nil, fmt.Errorf("assembling disclosed claims into vc: %w", err)
	}

	if subj, ok := newVCObj["credentialSubject"].(map[string]interface{}); ok {
		clearEmpty(subj)
	}

	vcBytes, err := json.Marshal(&newVCObj)
	if err != nil {
		return nil, fmt.Errorf("marshalling vc object to JSON: %w", err)
	}

	newVC, err := populateCredential(vcBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing new VC from JSON: %w", err)
	}

	return newVC, nil
}

// CreateDisplayCredentialMap creates, for SD-JWT credentials, a Credential whose selective-disclosure subject fields
// are replaced with the disclosure data.
//
// Options may be provided to filter the disclosures that will be included in the display credential. If a disclosure is
// not included, the associated claim will not be present in the returned credential.
//
// If the calling Credential is not an SD-JWT credential, this method returns the credential itself.
func (vc *Credential) CreateDisplayCredentialMap( // nolint:funlen,gocyclo
	opts ...DisplayCredentialOption,
) (map[string]interface{}, error) {
	options := &displayCredOpts{}

	for _, opt := range opts {
		opt(options)
	}

	if options.displayAll && len(options.displayGiven) > 0 {
		return nil, fmt.Errorf("incompatible options provided")
	}

	if vc.SDJWTHashAlg == "" || vc.JWT == "" {
		bytes, err := vc.MarshalJSON()
		if err != nil {
			return nil, err
		}

		return json2.ToMap(bytes)
	}

	credClaims, err := unmarshalJWSClaims(vc.JWT, false, nil)
	if err != nil {
		return nil, fmt.Errorf("unmarshal VC JWT claims: %w", err)
	}

	credClaims.refineFromJWTClaims()

	useDisclosures := filterDisclosureList(vc.SDJWTDisclosures, options)

	newVCObj, err := common.GetDisclosedClaims(useDisclosures, credClaims.VC)
	if err != nil {
		return nil, fmt.Errorf("assembling disclosed claims into vc: %w", err)
	}

	if subj, ok := newVCObj["credentialSubject"].(map[string]interface{}); ok {
		clearEmpty(subj)
	}

	return newVCObj, nil
}

func filterDisclosureList(disclosures []*common.DisclosureClaim, options *displayCredOpts) []*common.DisclosureClaim {
	if options.displayAll {
		return disclosures
	}

	displayGivenMap := map[string]struct{}{}

	for _, given := range options.displayGiven {
		displayGivenMap[given] = struct{}{}
	}

	var out []*common.DisclosureClaim

	for _, disclosure := range disclosures {
		if _, ok := displayGivenMap[disclosure.Name]; ok {
			out = append(out, disclosure)
		}
	}

	return out
}

func clearEmpty(claims map[string]interface{}) {
	for name, value := range claims {
		if valueObj, ok := value.(map[string]interface{}); ok {
			clearEmpty(valueObj)

			if len(valueObj) == 0 {
				delete(claims, name)
			}
		}
	}
}
