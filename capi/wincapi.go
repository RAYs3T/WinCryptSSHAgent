package capi

import (
	"crypto/x509"
	"fmt"
	"sort"
	"syscall"
	"time"
	"unsafe"

	"github.com/fullsailor/pkcs7"
)

const (
	ALG_RSA_SHA1RSA     = "1.2.840.113549.1.1.5"
	ALG_RSA_SHA256RSA   = "1.2.840.113549.1.1.11"
	ALG_RSA_SHA384RSA   = "1.2.840.113549.1.1.12"
	ALG_RSA_SHA512RSA   = "1.2.840.113549.1.1.13"
	ALG_ECDSA_SHA1      = "1.2.840.10045.4.1"
	ALG_ECDSA_SPECIFIED = "1.2.840.10045.4.3"
	ALG_ECDSA_SHA256    = "1.2.840.10045.4.3.2"
	ALG_ECDSA_SHA384    = "1.2.840.10045.4.3.3"
	ALG_ECDSA_SHA512    = "1.2.840.10045.4.3.4"

	NCRYPT_PIN_PROPERTY = "SmartCardPin"
)

const (
	AT_KEYEXCHANGE       = uint32(1)
	AT_SIGNATURE         = uint32(2)
	CERT_NCRYPT_KEY_SPEC = uint32(0xFFFFFFFF)

	X509_ASN_ENCODING                    = 0x1
	PKCS_7_ASN_ENCODING                  = 0x10000
	CRYPT_ACQUIRE_CACHE_FLAG             = uint32(0x00000001)
	CRYPT_ACQUIRE_SILENT_FLAG            = uint32(0x40)
	CRYPT_ACQUIRE_ONLY_NCRYPT_KEY_FLAG   = uint32(0x00040000)
	CRYPT_ACQUIRE_PREFER_NCRYPT_KEY_FLAG = uint32(0x00020000)
)

var (
	modcrypt32                            = syscall.NewLazyDLL("crypt32.dll")
	modncrypt                             = syscall.NewLazyDLL("ncrypt.dll")
	user32                                = syscall.NewLazyDLL("user32.dll")
	procCryptSignMessage                  = modcrypt32.NewProc("CryptSignMessage")
	procCertDuplicateCertificateContext   = modcrypt32.NewProc("CertDuplicateCertificateContext")
	procCertGetCertificateContextProperty = modcrypt32.NewProc("CertGetCertificateContextProperty")
	procCryptAcquireCertificatePrivateKey = modcrypt32.NewProc("CryptAcquireCertificatePrivateKey")
	procNCryptSetProperty                 = modncrypt.NewProc("NCryptSetProperty")
	procFindWindowExA                     = user32.NewProc("FindWindowExA")
	procSetForegroundWindow               = user32.NewProc("SetForegroundWindow")
)

var disablePINCache = true

type cryptoapiBlob struct {
	DataSize uint32
	Data     uintptr
}

type cryptAlgorithmIdentifier struct {
	ObjId      uintptr
	Parameters cryptoapiBlob
}

type cryptSignMessagePara struct {
	CbSize                  uint32
	MsgEncodingType         uint32
	SigningCert             uintptr
	HashAlgorithm           cryptAlgorithmIdentifier
	HashAuxInfo             uintptr
	MsgCertSize             uint32
	MsgCert                 uintptr
	MsgCrlSize              uint32
	MsgCrl                  uintptr
	AuthAttrSize            uint32
	AuthAttr                uintptr
	UnauthAttrSize          uint32
	UnauthAttr              uintptr
	Flags                   uint32
	InnerContentType        uint32
	HashEncryptionAlgorithm cryptAlgorithmIdentifier
	HashEncryptionAuxInfo   uintptr
}

func bringWinSecToFront() {
	str := "Windows Security"
	buf := append([]byte(str), 0)
	i := 100
	hwnd := uintptr(0)
	for ; i > 0; i-- {
		time.Sleep(time.Millisecond * 100)
		hwnd, _, _ = syscall.SyscallN(procFindWindowExA.Addr(), 0, 0, 0, uintptr(unsafe.Pointer(&buf[0])))
		if hwnd > 0 {
			break
		}
	}
	for ; i > 0; i-- {
		res, _, _ := syscall.SyscallN(procSetForegroundWindow.Addr(), hwnd)
		time.Sleep(time.Millisecond * 100)
		if res == 0 {
			return
		}
	}
}

func cryptSignMessage(para *cryptSignMessagePara, data []byte) (sign []byte, err error) {
	dataPtr := uintptr(unsafe.Pointer(&data[0]))
	dataSize := uint32(len(data))
	dataSizePtr := uintptr(unsafe.Pointer(&dataSize))
	result := make([]byte, 0x2000)
	size := uint32(0x2000)
	sizePtr := uintptr(unsafe.Pointer(&size))
	resultPtr := uintptr(unsafe.Pointer(&result[0]))
	go bringWinSecToFront()
	r0, _, e1 := syscall.Syscall9(
		procCryptSignMessage.Addr(),
		7,
		uintptr(unsafe.Pointer(para)),
		1,
		1,
		uintptr(unsafe.Pointer(&dataPtr)),
		dataSizePtr,
		resultPtr,
		sizePtr,
		0,
		0,
	)
	if e1 != syscall.Errno(0) {
		return nil, e1
	}
	if r0 == 0 {
		return nil, fmt.Errorf("failed to sign")
	}
	return result[:size], nil
}

func certDuplicateCertificateContext(context *syscall.CertContext) (uintptr, error) {
	r0, _, e1 := syscall.Syscall(procCertDuplicateCertificateContext.Addr(), 1, uintptr(unsafe.Pointer(context)), 0, 0)
	if e1 != syscall.Errno(0) {
		return 0, e1
	}
	return r0, nil
}

func certGetCertificateContextProperty(context *syscall.CertContext, dwPropId uint32) int {
	pvData := uint32(0)
	pvDataPtr := uintptr(unsafe.Pointer(&pvData))
	pcbData := uint32(4)
	pcbDataPtr := uintptr(unsafe.Pointer(&pcbData))
	r0, _, _ := syscall.Syscall6(procCertGetCertificateContextProperty.Addr(), 4, uintptr(unsafe.Pointer(context)), uintptr(dwPropId), pvDataPtr, pcbDataPtr, 0, 0)
	return int(r0)
}

func certGetCertificateContextStringProperty(context *syscall.CertContext, dwPropId uint32) string {
	pcbData := uint32(0)
	pcbDataPtr := uintptr(unsafe.Pointer(&pcbData))
	hasProp, _, _ := syscall.SyscallN(procCertGetCertificateContextProperty.Addr(), uintptr(unsafe.Pointer(context)), uintptr(dwPropId), uintptr(0), pcbDataPtr)
	if hasProp == 0 {
		return ""
	}
	pvData := make([]uint16, 1+pcbData/2)
	pvDataPtr := uintptr(unsafe.Pointer(&pvData[0]))
	success, _, _ := syscall.SyscallN(procCertGetCertificateContextProperty.Addr(), uintptr(unsafe.Pointer(context)), uintptr(dwPropId), pvDataPtr, pcbDataPtr)
	if success == 0 {
		return ""
	}
	return syscall.UTF16ToString(pvData)
}

// nCryptSetPropertyString sets a string value for a named property for a CNG key storage object.
func nCryptSetPropertyString(hObject uintptr, pszProperty string, pbInput string, dwFlags uint32) (err error) {

	pszPropertyPtr, _ := syscall.UTF16PtrFromString(pszProperty)

	dataPtr := uintptr(0)
	dataSize := uint32(0)
	if pbInput != "" {
		stringPtr, _ := syscall.UTF16PtrFromString(pbInput)
		dataPtr = uintptr(unsafe.Pointer(stringPtr))
		dataSize = uint32(len(pbInput))
	}
	dataSizePtr := uintptr(unsafe.Pointer(&dataSize))

	r0, _, e1 := syscall.Syscall6(procNCryptSetProperty.Addr(), 5,
		hObject,
		uintptr(unsafe.Pointer(pszPropertyPtr)),
		dataPtr,
		dataSizePtr,
		uintptr(dwFlags),
		0,
	)

	if r0 != 0 {
		return e1
	}

	if e1 != syscall.Errno(0) {
		return e1
	}
	return nil
}

// cryptAcquireCertificatePrivateKey obtains the private key for a certificateContext, returning a CNG NCRYPT_KEY_HANDLE
// or a HCRYPTPROV depending on the flags given.
func cryptAcquireCertificatePrivateKey(certContext uintptr, flags uint32) (provContext uintptr, err error) {
	pvParameters := uint32(0)
	phCryptProvOrNCryptKey := uintptr(0)
	pdwKeySpec := 0 // Can be 0, AT_KEYEXCHANGE, AT_SIGNATURE, or CERT_NCRYPT_KEY_SPEC
	pfCallerFreeProvOrNCryptKey := false

	r0, _, e1 := syscall.Syscall6(procCryptAcquireCertificatePrivateKey.Addr(), 6,
		certContext,
		uintptr(flags),
		uintptr(unsafe.Pointer(&pvParameters)),
		uintptr(unsafe.Pointer(&phCryptProvOrNCryptKey)),
		uintptr(unsafe.Pointer(&pdwKeySpec)),
		uintptr(unsafe.Pointer(&pfCallerFreeProvOrNCryptKey)),
	)

	if r0 == 0 {
		return 0, fmt.Errorf("r0 was 0")
	}

	if e1 != syscall.Errno(0) {
		return 0, e1
	}

	return phCryptProvOrNCryptKey, nil
}

type Certificate struct {
	userIndex   int
	certContext uintptr
	*x509.Certificate
}

func (s *Certificate) Free() error {
	return syscall.CertFreeCertificateContext((*syscall.CertContext)(unsafe.Pointer(s.certContext)))
}

func (s *Certificate) Copy() (*Certificate, error) {
	context := (*syscall.CertContext)(unsafe.Pointer(s.certContext))
	certContext, err := certDuplicateCertificateContext(context)
	if err != nil {
		return nil, err
	}
	return &Certificate{
		certContext: certContext,
		Certificate: s.Certificate,
	}, nil
}

func LoadUserCerts() ([]*Certificate, error) {
	const (
		CERT_STORE_PROV_SYSTEM_A       = 9
		CERT_SYSTEM_STORE_CURRENT_USER = 0x00010000
		CERT_STORE_READONLY_FLAG       = 0x00008000
		CRYPT_E_NOT_FOUND              = 0x80092004
		CERT_KEY_SPEC_PROP_ID          = 6
		CERT_FRIENDLY_NAME_PROP_ID     = 11
		CERT_DESCRIPTION_PROP_ID       = 13
	)
	ptr, _ := syscall.BytePtrFromString("My")
	store, err := syscall.CertOpenStore(
		CERT_STORE_PROV_SYSTEM_A,
		0,
		0,
		CERT_SYSTEM_STORE_CURRENT_USER|CERT_STORE_READONLY_FLAG,
		uintptr(unsafe.Pointer(ptr)),
	)
	if err != nil {
		return nil, err
	}
	defer syscall.CertCloseStore(store, 0)

	certs := make([]*Certificate, 0)
	var cert *syscall.CertContext
	var anyWithUserIndex bool = false
	for {
		cert, err = syscall.CertEnumCertificatesInStore(store, cert)
		if err != nil {
			if errno, ok := err.(syscall.Errno); ok {
				if errno == CRYPT_E_NOT_FOUND {
					break
				}
			}
			return nil, err
		}
		if cert == nil {
			break
		}
		// Check private key
		propID := certGetCertificateContextProperty(cert, CERT_KEY_SPEC_PROP_ID)
		if propID == 0 {
			continue
		}
		// acquireFlags := uint32(CRYPT_ACQUIRE_CACHE_FLAG)
		// nCryptHandle, err1 := cryptAcquireCertificatePrivateKey(uintptr(unsafe.Pointer(cert)), acquireFlags)
		// print(nCryptHandle)
		// print(err1)
		// print("\n")
		desc := certGetCertificateContextStringProperty(cert, CERT_DESCRIPTION_PROP_ID)
		var userIndex int = -1
		match, _ := fmt.Sscanf(desc, "WinCryptSSHAgent[%d]", &userIndex)
		anyWithUserIndex = anyWithUserIndex || match > 0
		// Copy the buf, since ParseCertificate does not create its own copy.
		buf := (*[1 << 20]byte)(unsafe.Pointer(cert.EncodedCert))[:]
		buf2 := make([]byte, cert.Length)
		copy(buf2, buf)
		if c, err := x509.ParseCertificate(buf2); err == nil {
			cc, err := certDuplicateCertificateContext(cert)
			if err != nil {
				continue
			}
			certs = append(certs, &Certificate{
				userIndex,
				cc,
				c,
			})
		}
	}

	if anyWithUserIndex {
		filteredCerts := make([]*Certificate, 0)
		for _, c := range certs {
			if c.userIndex >= 0 {
				filteredCerts = append(filteredCerts, c)
			}
		}
		sort.Slice(filteredCerts[:], func(a, b int) bool { return filteredCerts[a].userIndex < filteredCerts[b].userIndex })
		return filteredCerts, nil
	}
	return certs, nil
}

func Sign(alg string, cert *Certificate, data []byte) (*pkcs7.PKCS7, error) {
	var nCryptHandle uintptr

	if disablePINCache {
		var err error
		// Acquire a handle for the private key attached to this certificate
		acquireFlags := uint32(CRYPT_ACQUIRE_CACHE_FLAG | CRYPT_ACQUIRE_ONLY_NCRYPT_KEY_FLAG)
		nCryptHandle, err = cryptAcquireCertificatePrivateKey(cert.certContext, acquireFlags)
		if err != nil {
			return nil, err
		}
	}

	algptr, err := syscall.BytePtrFromString(alg)
	if err != nil {
		return nil, err
	}
	sign, err := cryptSignMessage(&cryptSignMessagePara{
		CbSize:                  uint32(unsafe.Sizeof(cryptSignMessagePara{})),
		MsgEncodingType:         X509_ASN_ENCODING | PKCS_7_ASN_ENCODING,
		SigningCert:             cert.certContext,
		HashAlgorithm:           cryptAlgorithmIdentifier{ObjId: uintptr(unsafe.Pointer(algptr))},
		HashEncryptionAlgorithm: cryptAlgorithmIdentifier{ObjId: uintptr(unsafe.Pointer(algptr))},
	}, data)
	if err != nil {
		return nil, err
	}

	if disablePINCache && nCryptHandle != 0 {
		// Set the PIN to NULL so we are prompted again
		err = nCryptSetPropertyString(nCryptHandle, NCRYPT_PIN_PROPERTY, "", 0)
		if err != nil {
			return nil, fmt.Errorf("Could not set NCRYPT_PIN_PROPERTY: %v\n", err)
		}
	}

	return pkcs7.Parse(sign)
}

func SetDisablePINCache(b bool) {
	disablePINCache = b
}
