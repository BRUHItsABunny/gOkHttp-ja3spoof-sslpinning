package gokhttp_ja3spoof_sslpinning

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	crypto_utils "github.com/BRUHItsABunny/crypto-utils"
	gokhttp_constants "github.com/BRUHItsABunny/gOkHttp/constants"
	"github.com/cornelk/hashmap"
	oohttp "github.com/ooni/oohttp"
	utls "github.com/refraction-networking/utls"
	"net"
	"net/http"
	"strings"
)

type SSLPin struct {
	SkipCA    bool
	Pins      *hashmap.Map[string, struct{}]
	Hostname  string
	Algorithm string
}

type SSLPinningOption struct {
	SSLPins *hashmap.Map[string, *SSLPin]
}

func NewSSLPinningOption() *SSLPinningOption {
	return &SSLPinningOption{SSLPins: hashmap.New[string, *SSLPin]()}
}

func (p *SSLPinningOption) Execute(client *http.Client) error {
	typedTransport, ok := client.Transport.(*oohttp.StdlibTransport)
	if ok {
		typedTransport.Transport.PostHandshakeCallback = p.verifyConnectionState
	}
	return nil
}

func (p *SSLPinningOption) GetPinsForHost(hostname string) (*SSLPin, error) {
	if pin, ok := p.SSLPins.Get(hostname); ok {
		return pin, nil
	}
	return nil, gokhttp_constants.ErrHostNotFound
}

func (p *SSLPinningOption) AddPin(hostname string, skipCA bool, pins ...string) error {
	var pinObj *SSLPin
	pinObj, ok := p.SSLPins.Get(hostname)
	if !ok {
		pinObj = &SSLPin{Hostname: hostname, SkipCA: skipCA, Pins: hashmap.New[string, struct{}]()}
	}
	for _, pin := range pins {
		step := strings.Split(pin, "\\")
		if pinObj.Algorithm == "" {
			pinObj.Algorithm = step[0]
		}
		if step[0] != pinObj.Algorithm {
			return gokhttp_constants.ErrUnmatchedAlgo
		}
		pinObj.Pins.Set(step[1], struct{}{})
	}
	p.SSLPins.Set(hostname, pinObj)
	return nil
}

func (p *SSLPinningOption) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	hostname := strings.Split(addr, ":")[0]
	pins, err := p.GetPinsForHost(hostname)
	if err != nil {
		if err != gokhttp_constants.ErrHostNotFound {
			panic(fmt.Errorf("pinner.GetPinsForHost: %w", err))
		}
		c, err := utls.Dial(network, addr, nil)
		if err != nil {
			return c, fmt.Errorf("utls.Dial: %w", err)
		}
		return c, nil
	} else {
		var acquiredPins []string
		acquiredPins = append(acquiredPins, "Chain for "+hostname+":")
		c, err := utls.Dial(network, addr, &utls.Config{InsecureSkipVerify: pins.SkipCA})
		if err != nil {
			return c, fmt.Errorf("tls.Dial: %w", err)
		}
		connState := c.ConnectionState()
		keyPinValid := false
		for _, peerCert := range connState.PeerCertificates {
			var hash []byte
			switch pins.Algorithm {
			case "sha256":
				hash = crypto_utils.SHA256hash(peerCert.RawSubjectPublicKeyInfo)
			case "sha1":
				hash = crypto_utils.SHA1hash(peerCert.RawSubjectPublicKeyInfo)
			default:
				panic("Unsupported algorithm")
			}
			acquiredPin := base64.StdEncoding.EncodeToString(hash)
			acquiredPins = append(acquiredPins, pinMessageFmt(pins.Algorithm+"\\"+acquiredPin, peerCert))
			_, keyPinValid = pins.Pins.Get(acquiredPin)
			if keyPinValid {
				break
			}
		}
		if keyPinValid == false {
			return c, errors.New("Insecure connection detected\n" + strings.Join(acquiredPins, "\n"))
		}
		return c, nil
	}
}

func (p *SSLPinningOption) verifyConnectionState(hostname string, connectionState *tls.ConnectionState) error {
	pins, err := p.GetPinsForHost(hostname)
	if err != nil {
		if !errors.Is(err, gokhttp_constants.ErrHostNotFound) {
			return fmt.Errorf("pinner.GetPinsForHost: %w", err)
		}
		return nil
	} else {
		var acquiredPins []string
		acquiredPins = append(acquiredPins, "Chain for "+hostname+":")
		keyPinValid := false
		for _, peerCert := range connectionState.PeerCertificates {
			var hash []byte
			switch pins.Algorithm {
			case "sha256":
				hash = crypto_utils.SHA256hash(peerCert.RawSubjectPublicKeyInfo)
			case "sha1":
				hash = crypto_utils.SHA1hash(peerCert.RawSubjectPublicKeyInfo)
			default:
				panic("Unsupported algorithm")
			}
			acquiredPin := base64.StdEncoding.EncodeToString(hash)
			acquiredPins = append(acquiredPins, pinMessageFmt(pins.Algorithm+"\\"+acquiredPin, peerCert))
			_, keyPinValid = pins.Pins.Get(acquiredPin)
			if keyPinValid {
				break
			}
		}
		if keyPinValid == false {
			return errors.New("Insecure connection detected\n" + strings.Join(acquiredPins, "\n"))
		}
		return nil
	}
}

func pinMessageFmt(acquiredPin string, peerCert *x509.Certificate) string {
	var pinMessage []string
	if len(peerCert.Subject.Country) > 0 {
		pinMessage = append(pinMessage, "C="+strings.Join(peerCert.Subject.Country, " "))
	}
	if len(peerCert.Subject.Province) > 0 {
		pinMessage = append(pinMessage, "ST="+strings.Join(peerCert.Subject.Province, " "))
	}
	if len(peerCert.Subject.Locality) > 0 {
		pinMessage = append(pinMessage, "L="+strings.Join(peerCert.Subject.Locality, " "))
	}
	if len(peerCert.Subject.Organization) > 0 {
		pinMessage = append(pinMessage, "O="+strings.Join(peerCert.Subject.Organization, " "))
	}
	if len(peerCert.Subject.OrganizationalUnit) > 0 {
		pinMessage = append(pinMessage, "OU="+strings.Join(peerCert.Subject.OrganizationalUnit, " "))
	}
	if peerCert.Subject.CommonName != "" {
		pinMessage = append(pinMessage, "CN="+peerCert.Subject.CommonName)
	}
	return fmt.Sprintf("%s:\n\t%s\t(valid from %s until %s)", strings.Join(pinMessage, "/"), acquiredPin, peerCert.NotBefore.String(), peerCert.NotAfter.String())
}
