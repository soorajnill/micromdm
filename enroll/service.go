package enroll

import (
	"golang.org/x/net/context"
	"io/ioutil"
)

type Service interface {
	Enroll(ctx context.Context) (Profile, error)
}

func NewService(pushCertPath string, pushCertPass string, caCertPath string, scepURL string, scepChallenge string, url string) (Service, error) {
	pushTopic, err := GetPushTopicFromPKCS12(pushCertPath, pushCertPass)
	if err != nil {
		return nil, err
	}

	var caCert []byte

	if caCertPath != "" {
		caCert, err = ioutil.ReadFile(caCertPath)

		if err != nil {
			return nil, err
		}
	}

	scepSubject := [][][]string{
		[][]string{
			[]string{"O", "MicroMDM"},
			[]string{"CN", "MDM Identity Certificate:UDID"},
		},
	}

	return &service{
		URL:           url,
		SCEPURL:       scepURL,
		SCEPSubject:   scepSubject,
		SCEPChallenge: scepChallenge,
		Topic:         pushTopic,
		CACert:        caCert,
	}, nil
}

type service struct {
	URL           string
	SCEPURL       string
	SCEPChallenge string
	SCEPSubject   [][][]string
	Topic         string // APNS Topic for MDM notifications
	CACert        []byte
}

func (svc service) Enroll(ctx context.Context) (Profile, error) {
	profile := NewProfile()
	profile.PayloadIdentifier = "com.github.micromdm.micromdm.mdm"
	profile.PayloadOrganization = "MicroMDM"
	profile.PayloadDisplayName = "Enrollment Profile"
	profile.PayloadDescription = "The server may alter your settings"
	profile.PayloadScope = "System"

	scepContent := SCEPPayloadContent{
		Challenge: svc.SCEPChallenge,
		URL:       svc.SCEPURL,
		Keysize:   1024,
		KeyType:   "RSA",
		KeyUsage:  0,
		Name:      "Device Management Identity Certificate",
		Subject:   svc.SCEPSubject,
	}

	scepPayload := NewPayload("com.apple.security.scep")
	scepPayload.PayloadDescription = "Configures SCEP"
	scepPayload.PayloadDisplayName = "SCEP"
	scepPayload.PayloadIdentifier = "com.github.micromdm.scep"
	scepPayload.PayloadOrganization = "MicroMDM"
	scepPayload.PayloadContent = scepContent
	scepPayload.PayloadScope = "System"

	mdmPayload := NewPayload("com.apple.mdm")
	mdmPayload.PayloadDescription = "Enrolls with the MDM server"
	mdmPayload.PayloadOrganization = "MicroMDM"
	mdmPayload.PayloadIdentifier = "com.github.micromdm.mdm"
	mdmPayload.PayloadScope = "System"

	mdmPayloadContent := MDMPayloadContent{
		Payload:                 *mdmPayload,
		AccessRights:            8191,
		CheckInURL:              svc.URL + "/mdm/checkin",
		CheckOutWhenRemoved:     true,
		ServerURL:               svc.URL + "/mdm/connect",
		IdentityCertificateUUID: scepPayload.PayloadUUID,
		Topic: svc.Topic,
	}

	if len(svc.CACert) > 0 {
		caPayload := NewPayload("com.apple.ssl.certificate")
		caPayload.PayloadDisplayName = "Root certificate for MicroMDM"
		caPayload.PayloadDescription = "Installs the root CA certificate for MicroMDM"
		caPayload.PayloadIdentifier = "com.github.micromdm.ssl.ca"
		caPayload.PayloadContent = svc.CACert

		profile.PayloadContent = []interface{}{*scepPayload, mdmPayloadContent, *caPayload}
	} else {
		profile.PayloadContent = []interface{}{*scepPayload, mdmPayloadContent}
	}

	return *profile, nil
}
