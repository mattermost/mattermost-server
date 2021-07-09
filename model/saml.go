// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"time"
)

const (
	UserAuthServiceSaml     = "saml"
	UserAuthServiceSamlText = "SAML"
	UserAuthServiceIsSaml   = "isSaml"
	UserAuthServiceIsMobile = "isMobile"
	UserAuthServiceIsOAuth  = "isOAuthUser"
)

type SamlAuthRequest struct {
	Base64AuthRequest string
	URL               string
	RelayState        string
}

type SamlCertificateStatus struct {
	IDpCertificateFile    bool `json:"idp_certificate_file"`
	PrivateKeyFile        bool `json:"private_key_file"`
	PublicCertificateFile bool `json:"public_certificate_file"`
}

type SamlMetadataResponse struct {
	IDpDescriptorURL     string `json:"idp_descriptor_url"`
	IDpURL               string `json:"idp_url"`
	IDpPublicCertificate string `json:"idp_public_certificate"`
}

type NameIDFormat struct {
	XMLName xml.Name
	Format  string `xml:",attr,omitempty"`
	Value   string `xml:",innerxml"`
}

type NameID struct {
	NameQualifier   string `xml:",attr"`
	SPNameQualifier string `xml:",attr"`
	Format          string `xml:",attr,omitempty"`
	SPProvidedID    string `xml:",attr"`
	Value           string `xml:",chardata"`
}

type AttributeValue struct {
	Type   string `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Value  string `xml:",chardata"`
	NameID *NameID
}

type Attribute struct {
	XMLName      xml.Name
	FriendlyName string           `xml:",attr"`
	Name         string           `xml:",attr"`
	NameFormat   string           `xml:",attr"`
	Values       []AttributeValue `xml:"AttributeValue"`
}

type Endpoint struct {
	XMLName          xml.Name
	Binding          string `xml:"Binding,attr"`
	Location         string `xml:"Location,attr"`
	ResponseLocation string `xml:"ResponseLocation,attr,omitempty"`
}

type IndexedEndpoint struct {
	XMLName          xml.Name
	Binding          string  `xml:"Binding,attr"`
	Location         string  `xml:"Location,attr"`
	ResponseLocation *string `xml:"ResponseLocation,attr,omitempty"`
	Index            int     `xml:"index,attr"`
	IsDefault        *bool   `xml:"isDefault,attr"`
}

type IDPSSODescriptor struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:metadata IDPSSODescriptor"`
	SSODescriptor
	WantAuthnRequestsSigned *bool `xml:",attr"`

	SingleSignOnServices       []Endpoint  `xml:"SingleSignOnService"`
	NameIDMappingServices      []Endpoint  `xml:"NameIDMappingService"`
	AssertionIDRequestServices []Endpoint  `xml:"AssertionIDRequestService"`
	AttributeProfiles          []string    `xml:"AttributeProfile"`
	Attributes                 []Attribute `xml:"Attribute"`
}

type SSODescriptor struct {
	XMLName xml.Name
	RoleDescriptor
	ArtifactResolutionServices []IndexedEndpoint `xml:"ArtifactResolutionService"`
	SingleLogoutServices       []Endpoint        `xml:"SingleLogoutService"`
	ManageNameIDServices       []Endpoint        `xml:"ManageNameIDService"`
	NameIDFormats              []NameIDFormat    `xml:"NameIDFormat"`
}

type X509Certificate struct {
	XMLName xml.Name
	Cert    string `xml:",innerxml"`
}

type X509Data struct {
	XMLName         xml.Name
	X509Certificate X509Certificate `xml:"X509Certificate"`
}

type KeyInfo struct {
	XMLName  xml.Name
	DS       string   `xml:"xmlns:ds,attr"`
	X509Data X509Data `xml:"X509Data"`
}
type EncryptionMethod struct {
	Algorithm string `xml:"Algorithm,attr"`
}

type KeyDescriptor struct {
	XMLName xml.Name
	Use     string  `xml:"use,attr,omitempty"`
	KeyInfo KeyInfo `xml:"http://www.w3.org/2000/09/xmldsig# KeyInfo,omitempty"`
}

type RoleDescriptor struct {
	XMLName                    xml.Name
	ID                         string          `xml:",attr,omitempty"`
	ValidUntil                 time.Time       `xml:"validUntil,attr,omitempty"`
	CacheDuration              time.Duration   `xml:"cacheDuration,attr,omitempty"`
	ProtocolSupportEnumeration string          `xml:"protocolSupportEnumeration,attr"`
	ErrorURL                   string          `xml:"errorURL,attr,omitempty"`
	KeyDescriptors             []KeyDescriptor `xml:"KeyDescriptor,omitempty"`
	Organization               *Organization   `xml:"Organization,omitempty"`
	ContactPersons             []ContactPerson `xml:"ContactPerson,omitempty"`
}

type ContactPerson struct {
	XMLName          xml.Name
	ContactType      string `xml:"contactType,attr"`
	Company          string
	GivenName        string
	SurName          string
	EmailAddresses   []string `xml:"EmailAddress"`
	TelephoneNumbers []string `xml:"TelephoneNumber"`
}

type LocalizedName struct {
	Lang  string `xml:"xml lang,attr"`
	Value string `xml:",chardata"`
}

type LocalizedURI struct {
	Lang  string `xml:"xml lang,attr"`
	Value string `xml:",chardata"`
}

type Organization struct {
	XMLName                  xml.Name
	OrganizationNames        []LocalizedName `xml:"OrganizationName"`
	OrganizationDisplayNames []LocalizedName `xml:"OrganizationDisplayName"`
	OrganizationURLs         []LocalizedURI  `xml:"OrganizationURL"`
}

type EntityDescriptor struct {
	XMLName           xml.Name           `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
	EntityID          string             `xml:"entityID,attr"`
	ID                string             `xml:",attr,omitempty"`
	ValidUntil        time.Time          `xml:"validUntil,attr,omitempty"`
	CacheDuration     time.Duration      `xml:"cacheDuration,attr,omitempty"`
	RoleDescriptors   []RoleDescriptor   `xml:"RoleDescriptor"`
	IDPSSODescriptors []IDPSSODescriptor `xml:"IDPSSODescriptor"`
	Organization      Organization       `xml:"Organization"`
	ContactPerson     ContactPerson      `xml:"ContactPerson"`
}

func (s *SamlCertificateStatus) ToJSON() string {
	b, _ := json.Marshal(s)
	return string(b)
}

func SamlCertificateStatusFromJSON(data io.Reader) *SamlCertificateStatus {
	var status *SamlCertificateStatus
	json.NewDecoder(data).Decode(&status)
	return status
}

func (s *SamlMetadataResponse) ToJSON() string {
	b, _ := json.Marshal(s)
	return string(b)
}

func SamlMetadataResponseFromJSON(data io.Reader) *SamlMetadataResponse {
	var status *SamlMetadataResponse
	json.NewDecoder(data).Decode(&status)
	return status
}
