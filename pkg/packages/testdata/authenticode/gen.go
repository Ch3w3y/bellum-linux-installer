//go:build ignore

// gen.go writes the throwaway test PKI and a minimal PE32+ image used by the
// Authenticode tests. Run gen.sh (which calls this and osslsigncode) to
// regenerate the fixtures; nothing here is used at run time.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"encoding/pem"
	"math/big"
	"os"
	"time"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func key() *rsa.PrivateKey {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	must(err)
	return k
}

func writePEM(name, typ string, der []byte) {
	f, err := os.Create(name)
	must(err)
	defer f.Close()
	must(pem.Encode(f, &pem.Block{Type: typ, Bytes: der}))
}

func writeKey(name string, k *rsa.PrivateKey) {
	der, err := x509.MarshalPKCS8PrivateKey(k)
	must(err)
	writePEM(name, "PRIVATE KEY", der)
}

var serial int64 = 1

func issue(tmpl *x509.Certificate, parent *x509.Certificate, pub, signer any) *x509.Certificate {
	serial++
	tmpl.SerialNumber = big.NewInt(serial)
	if parent == nil {
		parent = tmpl
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, signer)
	must(err)
	c, err := x509.ParseCertificate(der)
	must(err)
	return c
}

func leaf(name, cn string, ca *x509.Certificate, caKey *rsa.PrivateKey, from, to time.Time, eku []x509.ExtKeyUsage) {
	k := key()
	tmpl := &x509.Certificate{
		Subject:     pkix.Name{CommonName: cn, Organization: []string{"Bellum test fixtures"}},
		NotBefore:   from,
		NotAfter:    to,
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: eku,
	}
	if len(eku) == 1 && eku[0] == x509.ExtKeyUsageTimeStamping {
		// RFC 3161 requires the timeStamping EKU to be critical.
		value, err := asn1.Marshal([]asn1.ObjectIdentifier{{1, 3, 6, 1, 5, 5, 7, 3, 8}})
		must(err)
		tmpl.ExtraExtensions = []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Critical: true, Value: value}}
	}
	c := issue(tmpl, ca, &k.PublicKey, caKey)
	writePEM(name+".pem", "CERTIFICATE", c.Raw)
	writeKey(name+".key", k)
}

// minimalPE returns a 1 KiB PE32+ image: headers plus one .text section.
func minimalPE() []byte {
	b := make([]byte, 0x400)
	le := binary.LittleEndian
	copy(b, "MZ")
	le.PutUint32(b[0x3c:], 0x40)
	copy(b[0x40:], "PE\x00\x00")
	coff := 0x44
	le.PutUint16(b[coff:], 0x8664)  // AMD64
	le.PutUint16(b[coff+2:], 1)     // sections
	le.PutUint16(b[coff+16:], 0xF0) // optional header size
	le.PutUint16(b[coff+18:], 0x22) // executable, large address aware
	opt := coff + 20
	le.PutUint16(b[opt:], 0x20b)     // PE32+
	le.PutUint32(b[opt+4:], 0x200)   // SizeOfCode
	le.PutUint32(b[opt+16:], 0x1000) // AddressOfEntryPoint
	le.PutUint32(b[opt+20:], 0x1000) // BaseOfCode
	le.PutUint64(b[opt+24:], 0x140000000)
	le.PutUint32(b[opt+32:], 0x1000) // SectionAlignment
	le.PutUint32(b[opt+36:], 0x200)  // FileAlignment
	le.PutUint16(b[opt+40:], 6)      // OS major
	le.PutUint16(b[opt+48:], 6)      // subsystem major
	le.PutUint32(b[opt+56:], 0x2000) // SizeOfImage
	le.PutUint32(b[opt+60:], 0x200)  // SizeOfHeaders
	le.PutUint16(b[opt+68:], 3)      // console subsystem
	le.PutUint16(b[opt+70:], 0x8160)
	le.PutUint64(b[opt+72:], 0x100000)
	le.PutUint64(b[opt+80:], 0x1000)
	le.PutUint64(b[opt+88:], 0x100000)
	le.PutUint64(b[opt+96:], 0x1000)
	le.PutUint32(b[opt+108:], 16) // NumberOfRvaAndSizes
	sec := opt + 0xF0
	copy(b[sec:], ".text")
	le.PutUint32(b[sec+8:], 0x10)    // VirtualSize
	le.PutUint32(b[sec+12:], 0x1000) // VirtualAddress
	le.PutUint32(b[sec+16:], 0x200)  // SizeOfRawData
	le.PutUint32(b[sec+20:], 0x200)  // PointerToRawData
	le.PutUint32(b[sec+36:], 0x60000020)
	b[0x200] = 0xC3 // ret
	return b
}

func main() {
	far := time.Date(2045, 1, 1, 0, 0, 0, 0, time.UTC)
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	caKey := key()
	ca := issue(&x509.Certificate{
		Subject:               pkix.Name{CommonName: "Bellum Test Root CA"},
		NotBefore:             start,
		NotAfter:              far,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}, nil, &caKey.PublicKey, caKey)
	writePEM("ca.pem", "CERTIFICATE", ca.Raw)

	codeSigning := []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}
	leaf("signer", "ASTARTE INDUSTRIES INC.", ca, caKey, start, far, codeSigning)
	leaf("impostor", "ASTARTE INDUSTRIES INC. (not really)", ca, caKey, start, far, codeSigning)
	leaf("noeku", "ASTARTE INDUSTRIES INC.", ca, caKey, start, far, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	leaf("expired", "ASTARTE INDUSTRIES INC.", ca, caKey, start, time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), codeSigning)
	leaf("tsa", "Bellum Test TSA", ca, caKey, start, far, []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping})

	must(os.WriteFile("unsigned.exe", minimalPE(), 0644))
}
