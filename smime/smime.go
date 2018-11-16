//Package smime implants parts of the S/MIME 4.0 specification rfc5751-bis-12.
//
//See https://www.ietf.org/id/draft-ietf-lamps-rfc5751-bis-12.txt
package smime

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/InfiniteLoopSpace/go_S-MIME/b64"

	cms "github.com/InfiniteLoopSpace/go_S-MIME/cms"
	mime "github.com/InfiniteLoopSpace/go_S-MIME/mime"
)

// SMIME is an instance of cms to en-/decrypt and sign/verfiy SMIME messages
// with the given keyPairs and options.
type SMIME struct {
	CMS *cms.CMS
}

// New create a new instance of SMIME with given keyPairs.
func New(keyPair ...tls.Certificate) (smime *SMIME, err error) {
	CMS, err := cms.New(keyPair...)
	if err != nil {
		return
	}

	smime = &SMIME{CMS}

	return
}

// Decrypt decrypts SMIME message and returns plaintext.
func (smime *SMIME) Decrypt(msg []byte) (plaintext []byte, err error) {

	mail := mime.Parse(msg)

	mediaType, params, err := mail.ParseMediaType()

	if !strings.HasPrefix(mediaType, "application/pkcs7-mime") {
		err = errors.New("Unsupported media type: Can not decrypt this mail")
		return
	}

	if !strings.HasPrefix(params["smime-type"], "enveloped-data") {
		err = errors.New("Unsupported smime type: Can not decrypt this mail")
		return
	}

	contentTransferEncoding := mail.GetHeaderField([]byte("Content-Transfer-Encoding"))
	if len(contentTransferEncoding) != 1 && !strings.HasPrefix(string(contentTransferEncoding[0]), "base64") {
		err = errors.New("Unsupported endoing: Can not decrypt this mail. Only base64 is supported")
		return

	}

	bodyB64 := mail.Body()

	body := make([]byte, base64.StdEncoding.DecodedLen(len(bodyB64)))

	if _, err = base64.StdEncoding.Decode(body, bodyB64); err != nil {
		return
	}
	plaintext, err = smime.CMS.Decrypt(body)

	return
}

// Encrypt encrypts msg for the recipients and returns SMIME message.
func (smime *SMIME) Encrypt(msg []byte, recipients []*x509.Certificate, opts ...Header) (smimemsg []byte, err error) {

	mail := mime.Parse(msg)

	der, err := smime.CMS.Encrypt(msg, recipients)
	if err != nil {
		return
	}

	base64, err := b64.EncodeBase64(der)
	if err != nil {
		return
	}

	mail.SetBody(base64)

	for _, opt := range opts {
		mail.SetHeaderField([]byte(opt.Key), []byte(opt.Value))
	}

	contentType := []byte("application/pkcs7-mime; smime-type=enveloped-data;\n name=smime.p7m")
	contentTransferEncoding := []byte("base64")
	contentDisposition := []byte("attachment; filename=smime.p7m")
	mail.SetHeaderField([]byte("Content-Type"), contentType)
	mail.SetHeaderField([]byte("Content-Transfer-Encoding"), contentTransferEncoding)
	mail.SetHeaderField([]byte("Content-Disposition"), contentDisposition)

	return mail.Full(), nil
}

// AuthEncrypt authenticated-encrypts msg for the recipients and returns SMIME message.
func (smime *SMIME) AuthEncrypt(msg []byte, recipients []*x509.Certificate, opts ...Header) (smimemsg []byte, err error) {

	mail := mime.Parse(msg)

	der, err := smime.CMS.AuthEncrypt(msg, recipients)
	if err != nil {
		return
	}

	base64, err := b64.EncodeBase64(der)
	if err != nil {
		return
	}

	mail.SetBody(base64)

	for _, opt := range opts {
		mail.SetHeaderField([]byte(opt.Key), []byte(opt.Value))
	}

	contentType := []byte("application/pkcs7-mime; smime-type=authEnveloped-data;\n name=smime.p7m")
	contentTransferEncoding := []byte("base64")
	contentDisposition := []byte("attachment; filename=smime.p7m")
	mail.SetHeaderField([]byte("Content-Type"), contentType)
	mail.SetHeaderField([]byte("Content-Transfer-Encoding"), contentTransferEncoding)
	mail.SetHeaderField([]byte("Content-Disposition"), contentDisposition)

	return mail.Full(), nil
}

// Header field for creating signed or encrypted messages.
type Header struct {
	Key   string
	Value string
}

// Verify verifies a signed mail and returns certificate chains of the signers if
// the signature is valid.
func (smime *SMIME) Verify(msg []byte) (chains [][][]*x509.Certificate, err error) {

	mail := mime.Parse(msg)

	mediaType, params, err := mail.ParseMediaType()

	if !strings.HasPrefix(mediaType, "multipart/signed") {
		err = errors.New("Unsupported media type: can not decrypt this mail")
		return
	}

	if !strings.HasPrefix(params["protocol"], "application/pkcs7-signature") {
		err = errors.New("Unsupported smime type: can not decrypt this mail")
		return
	}

	parts, err := mail.MultipartGetParts()

	if len(parts) != 2 {
		err = errors.New("Multipart/signed Message must have 2 parts")
		return
	}

	signedMsg := parts[0].Bytes(mime.CRLF)

	signature := mime.Parse(parts[1].Bytes(nil))

	mediaType, params, err = signature.ParseMediaType()

	if !strings.HasPrefix(mediaType, "application/pkcs7-signature") {
		err = errors.New("Unsupported media type: Can not decrypt this mail")
		return
	}

	contentTransferEncoding := signature.GetHeaderField([]byte("Content-Transfer-Encoding"))

	var signatureDer []byte

	if len(contentTransferEncoding) == 1 {
		switch string(contentTransferEncoding[0]) {
		case "base64":
			signatureDer = make([]byte, base64.StdEncoding.DecodedLen(len(signature.Body())))

			if _, err = base64.StdEncoding.Decode(signatureDer, signature.Body()); err != nil {
				return
			}
		default:
			err = errors.New("Unsupported endoing: Can not parse the signature. Only base64 encoding is supported")
			return
		}

	} else {
		err = errors.New("Unsupported endoing: Multiple or no Content-Transfer-Encoding field")
		return
	}

	return smime.CMS.VerifyDetached(signatureDer, signedMsg)
}
