// Code generated by protoc-gen-go.
// source: AccessControl.proto
// DO NOT EDIT!

/*
Package proto is a generated protocol buffer package.

It is generated from these files:
	AccessControl.proto
	Admin.proto
	Aggregate.proto
	Authentication.proto
	Cell.proto
	Client.proto
	ClusterId.proto
	ClusterStatus.proto
	Comparator.proto
	Encryption.proto
	ErrorHandling.proto
	FS.proto
	Filter.proto
	HBase.proto
	HFile.proto
	LoadBalancer.proto
	MapReduce.proto
	Master.proto
	MultiRowMutation.proto
	RPC.proto
	RegionServerStatus.proto
	RowProcessor.proto
	SecureBulkLoad.proto
	Snapshot.proto
	Themis.proto
	Tracing.proto
	VisibilityLabels.proto
	WAL.proto
	ZooKeeper.proto

It has these top-level messages:
	Permission
	TablePermission
	NamespacePermission
	GlobalPermission
	UserPermission
	UsersAndPermissions
	GrantRequest
	GrantResponse
	RevokeRequest
	RevokeResponse
	GetUserPermissionsRequest
	GetUserPermissionsResponse
	CheckPermissionsRequest
	CheckPermissionsResponse
*/
package proto

import proto1 "github.com/golang/protobuf/proto"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto1.Marshal
var _ = math.Inf

type Permission_Action int32

const (
	Permission_READ   Permission_Action = 0
	Permission_WRITE  Permission_Action = 1
	Permission_EXEC   Permission_Action = 2
	Permission_CREATE Permission_Action = 3
	Permission_ADMIN  Permission_Action = 4
)

var Permission_Action_name = map[int32]string{
	0: "READ",
	1: "WRITE",
	2: "EXEC",
	3: "CREATE",
	4: "ADMIN",
}
var Permission_Action_value = map[string]int32{
	"READ":   0,
	"WRITE":  1,
	"EXEC":   2,
	"CREATE": 3,
	"ADMIN":  4,
}

func (x Permission_Action) Enum() *Permission_Action {
	p := new(Permission_Action)
	*p = x
	return p
}
func (x Permission_Action) String() string {
	return proto1.EnumName(Permission_Action_name, int32(x))
}
func (x *Permission_Action) UnmarshalJSON(data []byte) error {
	value, err := proto1.UnmarshalJSONEnum(Permission_Action_value, data, "Permission_Action")
	if err != nil {
		return err
	}
	*x = Permission_Action(value)
	return nil
}

type Permission_Type int32

const (
	Permission_Global    Permission_Type = 1
	Permission_Namespace Permission_Type = 2
	Permission_Table     Permission_Type = 3
)

var Permission_Type_name = map[int32]string{
	1: "Global",
	2: "Namespace",
	3: "Table",
}
var Permission_Type_value = map[string]int32{
	"Global":    1,
	"Namespace": 2,
	"Table":     3,
}

func (x Permission_Type) Enum() *Permission_Type {
	p := new(Permission_Type)
	*p = x
	return p
}
func (x Permission_Type) String() string {
	return proto1.EnumName(Permission_Type_name, int32(x))
}
func (x *Permission_Type) UnmarshalJSON(data []byte) error {
	value, err := proto1.UnmarshalJSONEnum(Permission_Type_value, data, "Permission_Type")
	if err != nil {
		return err
	}
	*x = Permission_Type(value)
	return nil
}

type Permission struct {
	Type                *Permission_Type     `protobuf:"varint,1,req,name=type,enum=proto.Permission_Type" json:"type,omitempty"`
	GlobalPermission    *GlobalPermission    `protobuf:"bytes,2,opt,name=global_permission" json:"global_permission,omitempty"`
	NamespacePermission *NamespacePermission `protobuf:"bytes,3,opt,name=namespace_permission" json:"namespace_permission,omitempty"`
	TablePermission     *TablePermission     `protobuf:"bytes,4,opt,name=table_permission" json:"table_permission,omitempty"`
	XXX_unrecognized    []byte               `json:"-"`
}

func (m *Permission) Reset()         { *m = Permission{} }
func (m *Permission) String() string { return proto1.CompactTextString(m) }
func (*Permission) ProtoMessage()    {}

func (m *Permission) GetType() Permission_Type {
	if m != nil && m.Type != nil {
		return *m.Type
	}
	return Permission_Global
}

func (m *Permission) GetGlobalPermission() *GlobalPermission {
	if m != nil {
		return m.GlobalPermission
	}
	return nil
}

func (m *Permission) GetNamespacePermission() *NamespacePermission {
	if m != nil {
		return m.NamespacePermission
	}
	return nil
}

func (m *Permission) GetTablePermission() *TablePermission {
	if m != nil {
		return m.TablePermission
	}
	return nil
}

type TablePermission struct {
	TableName        *TableName          `protobuf:"bytes,1,opt,name=table_name" json:"table_name,omitempty"`
	Family           []byte              `protobuf:"bytes,2,opt,name=family" json:"family,omitempty"`
	Qualifier        []byte              `protobuf:"bytes,3,opt,name=qualifier" json:"qualifier,omitempty"`
	Action           []Permission_Action `protobuf:"varint,4,rep,name=action,enum=proto.Permission_Action" json:"action,omitempty"`
	XXX_unrecognized []byte              `json:"-"`
}

func (m *TablePermission) Reset()         { *m = TablePermission{} }
func (m *TablePermission) String() string { return proto1.CompactTextString(m) }
func (*TablePermission) ProtoMessage()    {}

func (m *TablePermission) GetTableName() *TableName {
	if m != nil {
		return m.TableName
	}
	return nil
}

func (m *TablePermission) GetFamily() []byte {
	if m != nil {
		return m.Family
	}
	return nil
}

func (m *TablePermission) GetQualifier() []byte {
	if m != nil {
		return m.Qualifier
	}
	return nil
}

func (m *TablePermission) GetAction() []Permission_Action {
	if m != nil {
		return m.Action
	}
	return nil
}

type NamespacePermission struct {
	NamespaceName    []byte              `protobuf:"bytes,1,opt,name=namespace_name" json:"namespace_name,omitempty"`
	Action           []Permission_Action `protobuf:"varint,2,rep,name=action,enum=proto.Permission_Action" json:"action,omitempty"`
	XXX_unrecognized []byte              `json:"-"`
}

func (m *NamespacePermission) Reset()         { *m = NamespacePermission{} }
func (m *NamespacePermission) String() string { return proto1.CompactTextString(m) }
func (*NamespacePermission) ProtoMessage()    {}

func (m *NamespacePermission) GetNamespaceName() []byte {
	if m != nil {
		return m.NamespaceName
	}
	return nil
}

func (m *NamespacePermission) GetAction() []Permission_Action {
	if m != nil {
		return m.Action
	}
	return nil
}

type GlobalPermission struct {
	Action           []Permission_Action `protobuf:"varint,1,rep,name=action,enum=proto.Permission_Action" json:"action,omitempty"`
	XXX_unrecognized []byte              `json:"-"`
}

func (m *GlobalPermission) Reset()         { *m = GlobalPermission{} }
func (m *GlobalPermission) String() string { return proto1.CompactTextString(m) }
func (*GlobalPermission) ProtoMessage()    {}

func (m *GlobalPermission) GetAction() []Permission_Action {
	if m != nil {
		return m.Action
	}
	return nil
}

type UserPermission struct {
	User             []byte      `protobuf:"bytes,1,req,name=user" json:"user,omitempty"`
	Permission       *Permission `protobuf:"bytes,3,req,name=permission" json:"permission,omitempty"`
	XXX_unrecognized []byte      `json:"-"`
}

func (m *UserPermission) Reset()         { *m = UserPermission{} }
func (m *UserPermission) String() string { return proto1.CompactTextString(m) }
func (*UserPermission) ProtoMessage()    {}

func (m *UserPermission) GetUser() []byte {
	if m != nil {
		return m.User
	}
	return nil
}

func (m *UserPermission) GetPermission() *Permission {
	if m != nil {
		return m.Permission
	}
	return nil
}

// *
// Content of the /hbase/acl/<table or namespace> znode.
type UsersAndPermissions struct {
	UserPermissions  []*UsersAndPermissions_UserPermissions `protobuf:"bytes,1,rep,name=user_permissions" json:"user_permissions,omitempty"`
	XXX_unrecognized []byte                                 `json:"-"`
}

func (m *UsersAndPermissions) Reset()         { *m = UsersAndPermissions{} }
func (m *UsersAndPermissions) String() string { return proto1.CompactTextString(m) }
func (*UsersAndPermissions) ProtoMessage()    {}

func (m *UsersAndPermissions) GetUserPermissions() []*UsersAndPermissions_UserPermissions {
	if m != nil {
		return m.UserPermissions
	}
	return nil
}

type UsersAndPermissions_UserPermissions struct {
	User             []byte        `protobuf:"bytes,1,req,name=user" json:"user,omitempty"`
	Permissions      []*Permission `protobuf:"bytes,2,rep,name=permissions" json:"permissions,omitempty"`
	XXX_unrecognized []byte        `json:"-"`
}

func (m *UsersAndPermissions_UserPermissions) Reset()         { *m = UsersAndPermissions_UserPermissions{} }
func (m *UsersAndPermissions_UserPermissions) String() string { return proto1.CompactTextString(m) }
func (*UsersAndPermissions_UserPermissions) ProtoMessage()    {}

func (m *UsersAndPermissions_UserPermissions) GetUser() []byte {
	if m != nil {
		return m.User
	}
	return nil
}

func (m *UsersAndPermissions_UserPermissions) GetPermissions() []*Permission {
	if m != nil {
		return m.Permissions
	}
	return nil
}

type GrantRequest struct {
	UserPermission   *UserPermission `protobuf:"bytes,1,req,name=user_permission" json:"user_permission,omitempty"`
	XXX_unrecognized []byte          `json:"-"`
}

func (m *GrantRequest) Reset()         { *m = GrantRequest{} }
func (m *GrantRequest) String() string { return proto1.CompactTextString(m) }
func (*GrantRequest) ProtoMessage()    {}

func (m *GrantRequest) GetUserPermission() *UserPermission {
	if m != nil {
		return m.UserPermission
	}
	return nil
}

type GrantResponse struct {
	XXX_unrecognized []byte `json:"-"`
}

func (m *GrantResponse) Reset()         { *m = GrantResponse{} }
func (m *GrantResponse) String() string { return proto1.CompactTextString(m) }
func (*GrantResponse) ProtoMessage()    {}

type RevokeRequest struct {
	UserPermission   *UserPermission `protobuf:"bytes,1,req,name=user_permission" json:"user_permission,omitempty"`
	XXX_unrecognized []byte          `json:"-"`
}

func (m *RevokeRequest) Reset()         { *m = RevokeRequest{} }
func (m *RevokeRequest) String() string { return proto1.CompactTextString(m) }
func (*RevokeRequest) ProtoMessage()    {}

func (m *RevokeRequest) GetUserPermission() *UserPermission {
	if m != nil {
		return m.UserPermission
	}
	return nil
}

type RevokeResponse struct {
	XXX_unrecognized []byte `json:"-"`
}

func (m *RevokeResponse) Reset()         { *m = RevokeResponse{} }
func (m *RevokeResponse) String() string { return proto1.CompactTextString(m) }
func (*RevokeResponse) ProtoMessage()    {}

type GetUserPermissionsRequest struct {
	Type             *Permission_Type `protobuf:"varint,1,opt,name=type,enum=proto.Permission_Type" json:"type,omitempty"`
	TableName        *TableName       `protobuf:"bytes,2,opt,name=table_name" json:"table_name,omitempty"`
	NamespaceName    []byte           `protobuf:"bytes,3,opt,name=namespace_name" json:"namespace_name,omitempty"`
	XXX_unrecognized []byte           `json:"-"`
}

func (m *GetUserPermissionsRequest) Reset()         { *m = GetUserPermissionsRequest{} }
func (m *GetUserPermissionsRequest) String() string { return proto1.CompactTextString(m) }
func (*GetUserPermissionsRequest) ProtoMessage()    {}

func (m *GetUserPermissionsRequest) GetType() Permission_Type {
	if m != nil && m.Type != nil {
		return *m.Type
	}
	return Permission_Global
}

func (m *GetUserPermissionsRequest) GetTableName() *TableName {
	if m != nil {
		return m.TableName
	}
	return nil
}

func (m *GetUserPermissionsRequest) GetNamespaceName() []byte {
	if m != nil {
		return m.NamespaceName
	}
	return nil
}

type GetUserPermissionsResponse struct {
	UserPermission   []*UserPermission `protobuf:"bytes,1,rep,name=user_permission" json:"user_permission,omitempty"`
	XXX_unrecognized []byte            `json:"-"`
}

func (m *GetUserPermissionsResponse) Reset()         { *m = GetUserPermissionsResponse{} }
func (m *GetUserPermissionsResponse) String() string { return proto1.CompactTextString(m) }
func (*GetUserPermissionsResponse) ProtoMessage()    {}

func (m *GetUserPermissionsResponse) GetUserPermission() []*UserPermission {
	if m != nil {
		return m.UserPermission
	}
	return nil
}

type CheckPermissionsRequest struct {
	Permission       []*Permission `protobuf:"bytes,1,rep,name=permission" json:"permission,omitempty"`
	XXX_unrecognized []byte        `json:"-"`
}

func (m *CheckPermissionsRequest) Reset()         { *m = CheckPermissionsRequest{} }
func (m *CheckPermissionsRequest) String() string { return proto1.CompactTextString(m) }
func (*CheckPermissionsRequest) ProtoMessage()    {}

func (m *CheckPermissionsRequest) GetPermission() []*Permission {
	if m != nil {
		return m.Permission
	}
	return nil
}

type CheckPermissionsResponse struct {
	XXX_unrecognized []byte `json:"-"`
}

func (m *CheckPermissionsResponse) Reset()         { *m = CheckPermissionsResponse{} }
func (m *CheckPermissionsResponse) String() string { return proto1.CompactTextString(m) }
func (*CheckPermissionsResponse) ProtoMessage()    {}

func init() {
	proto1.RegisterEnum("proto.Permission_Action", Permission_Action_name, Permission_Action_value)
	proto1.RegisterEnum("proto.Permission_Type", Permission_Type_name, Permission_Type_value)
}
