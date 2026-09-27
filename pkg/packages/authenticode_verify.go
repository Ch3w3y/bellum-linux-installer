package packages

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"time"

	// Register the hashes Authenticode signatures use.
	_ "crypto/sha256"
	_ "crypto/sha512"
)

// This file verifies Windows Authenticode signatures in Go, so the installer
// no longer needs osslsigncode (which is AUR-only on Arch and cannot be
// installed on immutable hosts). It checks, in order:
//
//  1. the PE image digest recorded in the signature matches the file;
//  2. the PKCS#7 signer's authenticated attributes cover that content and
//     are signed by the signer certificate;
//  3. an RFC 3161 or legacy countersignature timestamp, when present and
//     valid, fixes the time used for certificate validity;
//  4. the signer chains to a trusted root with the code-signing EKU.
//
// Revocation is not checked (osslsigncode's CRL check also needed network
// access to each CA and failed closed without it).

var (
	oidSignedData          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidSpcIndirectData     = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 4}
	oidAttrContentType     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
	oidAttrMessageDigest   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	oidAttrSigningTime     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 5}
	oidAttrCounterSig      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 6}
	oidMSRFC3161Timestamp  = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 3, 3, 1}
	oidTSTInfo             = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 1, 4}
	oidSHA256              = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidSHA384              = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	oidSHA512              = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 3}
	oidRSAEncryption       = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	oidSHA256WithRSA       = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11}
	oidSHA384WithRSA       = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12}
	oidSHA512WithRSA       = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13}
	oidECPublicKey         = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
	oidECDSAWithSHA256     = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}
	oidECDSAWithSHA384     = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3}
	oidECDSAWithSHA512     = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 4}
	oidCommonName          = asn1.ObjectIdentifier{2, 5, 4, 3}
	maxAuthenticodeFileLen = 1 << 30
)

type contentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"explicit,optional,tag:0"`
}

type signedData struct {
	Version          int
	DigestAlgorithms []pkix.AlgorithmIdentifier `asn1:"set"`
	ContentInfo      contentInfo
	Certificates     asn1.RawValue `asn1:"optional,tag:0"`
	CRLs             asn1.RawValue `asn1:"optional,tag:1"`
	SignerInfos      []signerInfo  `asn1:"set"`
}

type issuerAndSerial struct {
	Issuer       asn1.RawValue
	SerialNumber *big.Int
}

type signerInfo struct {
	Version                   int
	IssuerAndSerial           issuerAndSerial
	DigestAlgorithm           pkix.AlgorithmIdentifier
	AuthenticatedAttributes   asn1.RawValue `asn1:"optional,tag:0"`
	DigestEncryptionAlgorithm pkix.AlgorithmIdentifier
	EncryptedDigest           []byte
	UnauthenticatedAttributes asn1.RawValue `asn1:"optional,tag:1"`
}

type attribute struct {
	Type   asn1.ObjectIdentifier
	Values asn1.RawValue `asn1:"set"`
}

type digestInfo struct {
	Algorithm pkix.AlgorithmIdentifier
	Digest    []byte
}

type spcIndirectDataContent struct {
	Data          asn1.RawValue
	MessageDigest digestInfo
}

type tstInfo struct {
	Version        int
	Policy         asn1.ObjectIdentifier
	MessageImprint digestInfo
	SerialNumber   *big.Int
	GenTime        time.Time `asn1:"generalized"`
}

// AuthenticodeSignature is what a successful verification established.
type AuthenticodeSignature struct {
	Signer *x509.Certificate
	// Timestamp is the verified countersignature time, or zero.
	Timestamp time.Time
}

// VerifyAuthenticode verifies the primary Authenticode signature of a PE
// image against roots at time now (or at the verified timestamp).
func VerifyAuthenticode(image []byte, roots *x509.CertPool, now time.Time) (*AuthenticodeSignature, error) {
	if len(image) > maxAuthenticodeFileLen {
		return nil, errors.New("file too large to verify")
	}
	pe, err := parsePESignatureLayout(image)
	if err != nil {
		return nil, err
	}
	sd, certs, err := parseSignedData(pe.signature)
	if err != nil {
		return nil, err
	}
	if !sd.ContentInfo.ContentType.Equal(oidSpcIndirectData) {
		return nil, errors.New("signature does not contain Authenticode data")
	}
	if len(sd.SignerInfos) != 1 {
		return nil, fmt.Errorf("expected one signer, found %d", len(sd.SignerInfos))
	}

	// The signed content is SpcIndirectDataContent; Authenticode hashes its
	// value octets (without the SEQUENCE tag and length).
	var contentSeq asn1.RawValue
	if rest, err := asn1.Unmarshal(sd.ContentInfo.Content.Bytes, &contentSeq); err != nil || len(rest) != 0 {
		return nil, errors.New("malformed Authenticode content")
	}
	var indirect spcIndirectDataContent
	if rest, err := asn1.Unmarshal(sd.ContentInfo.Content.Bytes, &indirect); err != nil || len(rest) != 0 {
		return nil, errors.New("malformed Authenticode content")
	}
	imageHash, err := hashForOID(indirect.MessageDigest.Algorithm.Algorithm)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(pe.digest(imageHash), indirect.MessageDigest.Digest) {
		return nil, errors.New("file digest does not match its signature (the file was modified)")
	}

	si := sd.SignerInfos[0]
	signer, err := verifySignerInfo(si, certs, contentSeq.Bytes, oidSpcIndirectData)
	if err != nil {
		return nil, err
	}

	result := &AuthenticodeSignature{Signer: signer}
	verifyAt := now
	if ts, err := verifyTimestamp(si, certs, roots); err == nil && !ts.IsZero() {
		result.Timestamp = ts
		verifyAt = ts
	} else if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	intermediates := x509.NewCertPool()
	for _, c := range certs {
		intermediates.AddCert(c)
	}
	if _, err := signer.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		CurrentTime:   verifyAt,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}); err != nil {
		return nil, fmt.Errorf("signer certificate is not trusted: %w", err)
	}
	return result, nil
}

// SignerCommonName returns the certificate's single CN, or an error when the
// subject has none or several (so "CN=A, CN=B" can't impersonate A).
func SignerCommonName(cert *x509.Certificate) (string, error) {
	var names []string
	for _, atv := range cert.Subject.Names {
		if atv.Type.Equal(oidCommonName) {
			s, ok := atv.Value.(string)
			if !ok {
				return "", errors.New("signer common name is not a string")
			}
			names = append(names, s)
		}
	}
	if len(names) != 1 {
		return "", fmt.Errorf("signer subject has %d common names", len(names))
	}
	return names[0], nil
}

type peSignatureLayout struct {
	image                     []byte
	checksumOff, certEntryOff int
	certTableOff              int
	signature                 []byte
}

// digest computes the Authenticode image hash: the whole file up to the
// certificate table, minus the checksum field and the certificate-table
// directory entry.
func (p peSignatureLayout) digest(h crypto.Hash) []byte {
	w := h.New()
	w.Write(p.image[:p.checksumOff])
	w.Write(p.image[p.checksumOff+4 : p.certEntryOff])
	w.Write(p.image[p.certEntryOff+8 : p.certTableOff])
	return w.Sum(nil)
}

func parsePESignatureLayout(b []byte) (peSignatureLayout, error) {
	le := binary.LittleEndian
	bad := func(msg string) (peSignatureLayout, error) {
		return peSignatureLayout{}, errors.New("not a signed Windows executable: " + msg)
	}
	if len(b) < 0x40 || b[0] != 'M' || b[1] != 'Z' {
		return bad("missing MZ header")
	}
	peOff := int(le.Uint32(b[0x3c:]))
	if peOff < 0x40 || peOff+24 > len(b) || !bytes.Equal(b[peOff:peOff+4], []byte("PE\x00\x00")) {
		return bad("missing PE header")
	}
	optOff := peOff + 24
	optSize := int(le.Uint16(b[peOff+20:]))
	if optOff+optSize > len(b) || optSize < 2 {
		return bad("truncated optional header")
	}
	var dirOff int
	switch le.Uint16(b[optOff:]) {
	case 0x10b: // PE32
		dirOff = optOff + 96
	case 0x20b: // PE32+
		dirOff = optOff + 112
	default:
		return bad("unknown optional header")
	}
	certEntryOff := dirOff + 4*8
	if certEntryOff+8 > optOff+optSize {
		return bad("no certificate table directory")
	}
	certOff := int(le.Uint32(b[certEntryOff:]))
	certSize := int(le.Uint32(b[certEntryOff+4:]))
	if certOff == 0 || certSize < 8 {
		return bad("no signature")
	}
	// The table must be the last thing in the file, so nothing unsigned can
	// ride along after it.
	if certOff < certEntryOff+8 || certOff+certSize != len(b) {
		return bad("certificate table is not at the end of the file")
	}
	winLen := int(le.Uint32(b[certOff:]))
	if le.Uint16(b[certOff+4:]) != 0x0200 || le.Uint16(b[certOff+6:]) != 0x0002 {
		return bad("unsupported certificate type")
	}
	if winLen < 8 || winLen > certSize {
		return bad("malformed certificate entry")
	}
	return peSignatureLayout{
		image:        b,
		checksumOff:  optOff + 64,
		certEntryOff: certEntryOff,
		certTableOff: certOff,
		signature:    b[certOff+8 : certOff+winLen],
	}, nil
}

func parseSignedData(der []byte) (signedData, []*x509.Certificate, error) {
	var ci contentInfo
	rest, err := asn1.Unmarshal(der, &ci)
	if err != nil {
		return signedData{}, nil, fmt.Errorf("malformed signature: %w", err)
	}
	// Signers pad the entry to 8 bytes with zeros.
	for _, c := range rest {
		if c != 0 {
			return signedData{}, nil, errors.New("unexpected data after signature")
		}
	}
	if !ci.ContentType.Equal(oidSignedData) {
		return signedData{}, nil, errors.New("signature is not PKCS#7 SignedData")
	}
	var sd signedData
	if rest, err := asn1.Unmarshal(ci.Content.Bytes, &sd); err != nil || len(rest) != 0 {
		return signedData{}, nil, errors.New("malformed PKCS#7 SignedData")
	}
	certs, err := x509.ParseCertificates(sd.Certificates.Bytes)
	if err != nil {
		return signedData{}, nil, fmt.Errorf("malformed certificates in signature: %w", err)
	}
	return sd, certs, nil
}

// verifySignerInfo checks that si's authenticated attributes bind content
// (and, when contentType is set, its type) and that the signer certificate
// from certs signed them. It returns that certificate.
func verifySignerInfo(si signerInfo, certs []*x509.Certificate, content []byte, contentType asn1.ObjectIdentifier) (*x509.Certificate, error) {
	var signer *x509.Certificate
	for _, c := range certs {
		if bytes.Equal(c.RawIssuer, si.IssuerAndSerial.Issuer.FullBytes) && c.SerialNumber.Cmp(si.IssuerAndSerial.SerialNumber) == 0 {
			signer = c
			break
		}
	}
	if signer == nil {
		return nil, errors.New("signer certificate is missing from the signature")
	}
	h, err := hashForOID(si.DigestAlgorithm.Algorithm)
	if err != nil {
		return nil, err
	}
	if len(si.AuthenticatedAttributes.Bytes) == 0 {
		return nil, errors.New("signature has no authenticated attributes")
	}
	attrs, err := parseAttributes(si.AuthenticatedAttributes.Bytes)
	if err != nil {
		return nil, err
	}
	var digest []byte
	if !attrOctets(attrs, oidAttrMessageDigest, &digest) {
		return nil, errors.New("signature lacks a message digest")
	}
	w := h.New()
	w.Write(content)
	if !bytes.Equal(digest, w.Sum(nil)) {
		return nil, errors.New("signature does not cover the signed content")
	}
	if contentType != nil {
		var got asn1.ObjectIdentifier
		if !attrValue(attrs, oidAttrContentType, &got) || !got.Equal(contentType) {
			return nil, errors.New("signature content type mismatch")
		}
	}
	// The signature covers the attributes DER-encoded as a SET OF.
	signed, err := asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSet, IsCompound: true, Bytes: si.AuthenticatedAttributes.Bytes})
	if err != nil {
		return nil, err
	}
	alg, err := signatureAlgorithm(si.DigestEncryptionAlgorithm.Algorithm, h)
	if err != nil {
		return nil, err
	}
	if err := signer.CheckSignature(alg, signed, si.EncryptedDigest); err != nil {
		return nil, fmt.Errorf("signature is invalid: %w", err)
	}
	return signer, nil
}

// verifyTimestamp returns the verified time of an RFC 3161 or legacy
// countersignature on si, zero when there is none, or an error when one is
// present but invalid.
func verifyTimestamp(si signerInfo, certs []*x509.Certificate, roots *x509.CertPool) (time.Time, error) {
	if len(si.UnauthenticatedAttributes.Bytes) == 0 {
		return time.Time{}, nil
	}
	attrs, err := parseAttributes(si.UnauthenticatedAttributes.Bytes)
	if err != nil {
		return time.Time{}, err
	}
	for _, a := range attrs {
		switch {
		case a.Type.Equal(oidMSRFC3161Timestamp):
			return verifyRFC3161(a.Values.Bytes, si.EncryptedDigest, roots)
		case a.Type.Equal(oidAttrCounterSig):
			var cs signerInfo
			if rest, err := asn1.Unmarshal(a.Values.Bytes, &cs); err != nil || len(rest) != 0 {
				return time.Time{}, errors.New("malformed countersignature")
			}
			tsa, err := verifySignerInfo(cs, certs, si.EncryptedDigest, nil)
			if err != nil {
				return time.Time{}, err
			}
			csAttrs, err := parseAttributes(cs.AuthenticatedAttributes.Bytes)
			if err != nil {
				return time.Time{}, err
			}
			var at time.Time
			if !attrValue(csAttrs, oidAttrSigningTime, &at) {
				return time.Time{}, errors.New("countersignature has no signing time")
			}
			return at, verifyTSA(tsa, certs, roots, at)
		}
	}
	return time.Time{}, nil
}

func verifyRFC3161(token, signature []byte, roots *x509.CertPool) (time.Time, error) {
	sd, certs, err := parseSignedData(token)
	if err != nil {
		return time.Time{}, err
	}
	if !sd.ContentInfo.ContentType.Equal(oidTSTInfo) || len(sd.SignerInfos) != 1 {
		return time.Time{}, errors.New("malformed timestamp token")
	}
	var tstDER []byte
	if rest, err := asn1.Unmarshal(sd.ContentInfo.Content.Bytes, &tstDER); err != nil || len(rest) != 0 {
		return time.Time{}, errors.New("malformed timestamp content")
	}
	tsa, err := verifySignerInfo(sd.SignerInfos[0], certs, tstDER, oidTSTInfo)
	if err != nil {
		return time.Time{}, err
	}
	var info tstInfo
	if _, err := asn1.Unmarshal(tstDER, &info); err != nil {
		return time.Time{}, errors.New("malformed timestamp info")
	}
	h, err := hashForOID(info.MessageImprint.Algorithm.Algorithm)
	if err != nil {
		return time.Time{}, err
	}
	w := h.New()
	w.Write(signature)
	if !bytes.Equal(w.Sum(nil), info.MessageImprint.Digest) {
		return time.Time{}, errors.New("timestamp is for a different signature")
	}
	return info.GenTime, verifyTSA(tsa, certs, roots, info.GenTime)
}

func verifyTSA(tsa *x509.Certificate, certs []*x509.Certificate, roots *x509.CertPool, at time.Time) error {
	intermediates := x509.NewCertPool()
	for _, c := range certs {
		intermediates.AddCert(c)
	}
	if _, err := tsa.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		CurrentTime:   at,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
	}); err != nil {
		return fmt.Errorf("timestamp authority is not trusted: %w", err)
	}
	return nil
}

func parseAttributes(b []byte) ([]attribute, error) {
	var attrs []attribute
	for len(b) > 0 {
		var a attribute
		rest, err := asn1.Unmarshal(b, &a)
		if err != nil {
			return nil, errors.New("malformed signature attributes")
		}
		attrs = append(attrs, a)
		b = rest
	}
	return attrs, nil
}

// attrValue decodes the single value of the attribute with the given type.
func attrValue(attrs []attribute, oid asn1.ObjectIdentifier, out any) bool {
	for _, a := range attrs {
		if a.Type.Equal(oid) {
			rest, err := asn1.Unmarshal(a.Values.Bytes, out)
			return err == nil && len(rest) == 0
		}
	}
	return false
}

func attrOctets(attrs []attribute, oid asn1.ObjectIdentifier, out *[]byte) bool {
	return attrValue(attrs, oid, out)
}

func hashForOID(oid asn1.ObjectIdentifier) (crypto.Hash, error) {
	switch {
	case oid.Equal(oidSHA256):
		return crypto.SHA256, nil
	case oid.Equal(oidSHA384):
		return crypto.SHA384, nil
	case oid.Equal(oidSHA512):
		return crypto.SHA512, nil
	}
	return 0, fmt.Errorf("unsupported or insecure digest algorithm %v", oid)
}

func signatureAlgorithm(oid asn1.ObjectIdentifier, h crypto.Hash) (x509.SignatureAlgorithm, error) {
	rsa := map[crypto.Hash]x509.SignatureAlgorithm{crypto.SHA256: x509.SHA256WithRSA, crypto.SHA384: x509.SHA384WithRSA, crypto.SHA512: x509.SHA512WithRSA}
	ec := map[crypto.Hash]x509.SignatureAlgorithm{crypto.SHA256: x509.ECDSAWithSHA256, crypto.SHA384: x509.ECDSAWithSHA384, crypto.SHA512: x509.ECDSAWithSHA512}
	switch {
	case oid.Equal(oidRSAEncryption), oid.Equal(oidSHA256WithRSA), oid.Equal(oidSHA384WithRSA), oid.Equal(oidSHA512WithRSA):
		return rsa[h], nil
	case oid.Equal(oidECPublicKey), oid.Equal(oidECDSAWithSHA256), oid.Equal(oidECDSAWithSHA384), oid.Equal(oidECDSAWithSHA512):
		return ec[h], nil
	}
	return 0, fmt.Errorf("unsupported signature algorithm %v", oid)
}
