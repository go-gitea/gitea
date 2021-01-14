package websspi

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// secur32.dll

type SECURITY_STATUS syscall.Errno

const (
	SEC_E_OK = SECURITY_STATUS(0)

	SEC_E_INCOMPLETE_MESSAGE          = SECURITY_STATUS(0x80090318)
	SEC_E_INSUFFICIENT_MEMORY         = SECURITY_STATUS(0x80090300)
	SEC_E_INTERNAL_ERROR              = SECURITY_STATUS(0x80090304)
	SEC_E_INVALID_HANDLE              = SECURITY_STATUS(0x80090301)
	SEC_E_INVALID_TOKEN               = SECURITY_STATUS(0x80090308)
	SEC_E_LOGON_DENIED                = SECURITY_STATUS(0x8009030C)
	SEC_E_NO_AUTHENTICATING_AUTHORITY = SECURITY_STATUS(0x80090311)
	SEC_E_NO_CREDENTIALS              = SECURITY_STATUS(0x8009030E)
	SEC_E_UNSUPPORTED_FUNCTION        = SECURITY_STATUS(0x80090302)
	SEC_I_COMPLETE_AND_CONTINUE       = SECURITY_STATUS(0x00090314)
	SEC_I_COMPLETE_NEEDED             = SECURITY_STATUS(0x00090313)
	SEC_I_CONTINUE_NEEDED             = SECURITY_STATUS(0x00090312)
	SEC_E_NOT_OWNER                   = SECURITY_STATUS(0x80090306)
	SEC_E_SECPKG_NOT_FOUND            = SECURITY_STATUS(0x80090305)
	SEC_E_UNKNOWN_CREDENTIALS         = SECURITY_STATUS(0x8009030D)

	NEGOSSP_NAME         = "Negotiate"
	SECPKG_CRED_INBOUND  = 1
	SECURITY_NATIVE_DREP = 16

	ASC_REQ_DELEGATE        = 1
	ASC_REQ_MUTUAL_AUTH     = 2
	ASC_REQ_REPLAY_DETECT   = 4
	ASC_REQ_SEQUENCE_DETECT = 8
	ASC_REQ_CONFIDENTIALITY = 16
	ASC_REQ_USE_SESSION_KEY = 32
	ASC_REQ_ALLOCATE_MEMORY = 256
	ASC_REQ_USE_DCE_STYLE   = 512
	ASC_REQ_DATAGRAM        = 1024
	ASC_REQ_CONNECTION      = 2048
	ASC_REQ_EXTENDED_ERROR  = 32768
	ASC_REQ_STREAM          = 65536
	ASC_REQ_INTEGRITY       = 131072

	SECPKG_ATTR_SIZES            = 0
	SECPKG_ATTR_NAMES            = 1
	SECPKG_ATTR_LIFESPAN         = 2
	SECPKG_ATTR_DCE_INFO         = 3
	SECPKG_ATTR_STREAM_SIZES     = 4
	SECPKG_ATTR_KEY_INFO         = 5
	SECPKG_ATTR_AUTHORITY        = 6
	SECPKG_ATTR_PROTO_INFO       = 7
	SECPKG_ATTR_PASSWORD_EXPIRY  = 8
	SECPKG_ATTR_SESSION_KEY      = 9
	SECPKG_ATTR_PACKAGE_INFO     = 10
	SECPKG_ATTR_USER_FLAGS       = 11
	SECPKG_ATTR_NEGOTIATION_INFO = 12
	SECPKG_ATTR_NATIVE_NAMES     = 13
	SECPKG_ATTR_FLAGS            = 14

	SECBUFFER_VERSION = 0
	SECBUFFER_TOKEN   = 2
)

type CredHandle struct {
	Lower uintptr
	Upper uintptr
}

type CtxtHandle struct {
	Lower uintptr
	Upper uintptr
}

type SecBuffer struct {
	BufferSize uint32
	BufferType uint32
	Buffer     *byte
}

type SecBufferDesc struct {
	Version      uint32
	BuffersCount uint32
	Buffers      *SecBuffer
}

type LUID struct {
	LowPart  uint32
	HighPart int32
}

type SecPkgContext_Names struct {
	UserName *uint16
}

type SecPkgContext_Flags struct {
	Flags uint32
}

// netapi32.dll

const (
	NERR_Success       = 0x0
	NERR_InternalError = 0x85C
	NERR_UserNotFound  = 0x8AD

	ERROR_ACCESS_DENIED     = 0x5
	ERROR_BAD_NETPATH       = 0x35
	ERROR_INVALID_LEVEL     = 0x7C
	ERROR_INVALID_NAME      = 0x7B
	ERROR_MORE_DATA         = 0xEA
	ERROR_NOT_ENOUGH_MEMORY = 0x8

	MAX_PREFERRED_LENGTH  = 0xFFFFFFFF
	MAX_GROUP_NAME_LENGTH = 256

	SE_GROUP_MANDATORY          = 0x1
	SE_GROUP_ENABLED_BY_DEFAULT = 0x2
	SE_GROUP_ENABLED            = 0x4
	SE_GROUP_OWNER              = 0x8
	SE_GROUP_USE_FOR_DENY_ONLY  = 0x10
	SE_GROUP_INTEGRITY          = 0x20
	SE_GROUP_INTEGRITY_ENABLED  = 0x40
	SE_GROUP_LOGON_ID           = 0xC0000000
	SE_GROUP_RESOURCE           = 0x20000000
)

type GroupUsersInfo0 struct {
	Grui0_name *uint16
}

type GroupUsersInfo1 struct {
	Grui1_name       *uint16
	Grui1_attributes uint32
}

// The API interface describes the Win32 functions used in this package and
// its primary purpose is to allow replacing them with stub functions in unit tests.
type API interface {
	AcquireCredentialsHandle(
		principal *uint16,
		_package *uint16,
		credentialUse uint32,
		logonID *LUID,
		authData *byte,
		getKeyFn uintptr,
		getKeyArgument uintptr,
		credHandle *CredHandle,
		expiry *syscall.Filetime,
	) SECURITY_STATUS
	AcceptSecurityContext(
		credential *CredHandle,
		context *CtxtHandle,
		input *SecBufferDesc,
		contextReq uint32,
		targDataRep uint32,
		newContext *CtxtHandle,
		output *SecBufferDesc,
		contextAttr *uint32,
		expiry *syscall.Filetime,
	) SECURITY_STATUS
	QueryContextAttributes(context *CtxtHandle, attribute uint32, buffer *byte) SECURITY_STATUS
	DeleteSecurityContext(context *CtxtHandle) SECURITY_STATUS
	FreeContextBuffer(buffer *byte) SECURITY_STATUS
	FreeCredentialsHandle(handle *CredHandle) SECURITY_STATUS
	NetUserGetGroups(
		serverName *uint16,
		userName *uint16,
		level uint32,
		buf **byte,
		prefmaxlen uint32,
		entriesread *uint32,
		totalentries *uint32,
	) (neterr error)
	NetApiBufferFree(buf *byte) (neterr error)
}

// Win32 implements the API interface by calling the relevant system functions
// from secur32.dll and netapi32.dll
type Win32 struct{}

var (
	secur32dll  = windows.NewLazySystemDLL("secur32.dll")
	netapi32dll = windows.NewLazySystemDLL("netapi32.dll")

	procAcquireCredentialsHandleW = secur32dll.NewProc("AcquireCredentialsHandleW")
	procAcceptSecurityContext     = secur32dll.NewProc("AcceptSecurityContext")
	procQueryContextAttributesW   = secur32dll.NewProc("QueryContextAttributesW")
	procDeleteSecurityContext     = secur32dll.NewProc("DeleteSecurityContext")
	procFreeContextBuffer         = secur32dll.NewProc("FreeContextBuffer")
	procFreeCredentialsHandle     = secur32dll.NewProc("FreeCredentialsHandle")
	procNetUserGetGroups          = netapi32dll.NewProc("NetUserGetGroups")
)

func (w *Win32) AcquireCredentialsHandle(
	principal *uint16,
	_package *uint16,
	credentialUse uint32,
	logonId *LUID,
	authData *byte,
	getKeyFn uintptr,
	getKeyArgument uintptr,
	credHandle *CredHandle,
	expiry *syscall.Filetime,
) SECURITY_STATUS {
	r1, _, _ := syscall.Syscall9(
		procAcquireCredentialsHandleW.Addr(), 9,
		uintptr(unsafe.Pointer(principal)),
		uintptr(unsafe.Pointer(_package)),
		uintptr(credentialUse),
		uintptr(unsafe.Pointer(logonId)),
		uintptr(unsafe.Pointer(authData)),
		uintptr(getKeyFn),
		uintptr(getKeyArgument),
		uintptr(unsafe.Pointer(credHandle)),
		uintptr(unsafe.Pointer(expiry)),
	)
	return SECURITY_STATUS(r1)
}

func (w *Win32) AcceptSecurityContext(
	credential *CredHandle,
	context *CtxtHandle,
	input *SecBufferDesc,
	contextReq uint32,
	targDataRep uint32,
	newContext *CtxtHandle,
	output *SecBufferDesc,
	contextAttr *uint32,
	expiry *syscall.Filetime,
) SECURITY_STATUS {
	r1, _, _ := syscall.Syscall9(
		procAcceptSecurityContext.Addr(), 9,
		uintptr(unsafe.Pointer(credential)),
		uintptr(unsafe.Pointer(context)),
		uintptr(unsafe.Pointer(input)),
		uintptr(contextReq),
		uintptr(targDataRep),
		uintptr(unsafe.Pointer(newContext)),
		uintptr(unsafe.Pointer(output)),
		uintptr(unsafe.Pointer(contextAttr)),
		uintptr(unsafe.Pointer(expiry)),
	)
	return SECURITY_STATUS(r1)
}

func (w *Win32) QueryContextAttributes(
	context *CtxtHandle,
	attribute uint32,
	buffer *byte,
) SECURITY_STATUS {
	r1, _, _ := syscall.Syscall(
		procQueryContextAttributesW.Addr(), 3,
		uintptr(unsafe.Pointer(context)),
		uintptr(attribute),
		uintptr(unsafe.Pointer(buffer)),
	)
	return SECURITY_STATUS(r1)
}

func (w *Win32) DeleteSecurityContext(context *CtxtHandle) SECURITY_STATUS {
	r1, _, _ := syscall.Syscall(
		procDeleteSecurityContext.Addr(), 1,
		uintptr(unsafe.Pointer(context)),
		0, 0,
	)
	return SECURITY_STATUS(r1)
}

func (w *Win32) FreeContextBuffer(buffer *byte) SECURITY_STATUS {
	r1, _, _ := syscall.Syscall(
		procFreeContextBuffer.Addr(), 1,
		uintptr(unsafe.Pointer(buffer)),
		0, 0,
	)
	return SECURITY_STATUS(r1)
}

func (w *Win32) FreeCredentialsHandle(handle *CredHandle) SECURITY_STATUS {
	r1, _, _ := syscall.Syscall(
		procFreeCredentialsHandle.Addr(), 1,
		uintptr(unsafe.Pointer(handle)),
		0, 0,
	)
	return SECURITY_STATUS(r1)
}

func (w *Win32) NetUserGetGroups(
	serverName *uint16,
	userName *uint16,
	level uint32,
	buf **byte,
	prefmaxlen uint32,
	entriesread *uint32,
	totalentries *uint32,
) (neterr error) {
	r0, _, _ := syscall.Syscall9(procNetUserGetGroups.Addr(), 7, uintptr(unsafe.Pointer(serverName)), uintptr(unsafe.Pointer(userName)), uintptr(level), uintptr(unsafe.Pointer(buf)), uintptr(prefmaxlen), uintptr(unsafe.Pointer(entriesread)), uintptr(unsafe.Pointer(totalentries)), 0, 0)
	if r0 != 0 {
		neterr = syscall.Errno(r0)
	}
	return
}

func (w *Win32) NetApiBufferFree(buf *byte) (neterr error) {
	return syscall.NetApiBufferFree(buf)
}
