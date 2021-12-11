package gomatrix

// ReqRegister is the JSON request for http://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-client-r0-register
type ReqRegister struct {
	Username                 string      `json:"username,omitempty"`
	BindEmail                bool        `json:"bind_email,omitempty"`
	Password                 string      `json:"password,omitempty"`
	DeviceID                 string      `json:"device_id,omitempty"`
	InitialDeviceDisplayName string      `json:"initial_device_display_name"`
	Auth                     interface{} `json:"auth,omitempty"`
}

// ReqLogin is the JSON request for http://matrix.org/docs/spec/client_server/r0.6.0.html#post-matrix-client-r0-login
type ReqLogin struct {
	Type                     string     `json:"type"`
	Identifier               Identifier `json:"identifier,omitempty"`
	Password                 string     `json:"password,omitempty"`
	Medium                   string     `json:"medium,omitempty"`
	User                     string     `json:"user,omitempty"`
	Address                  string     `json:"address,omitempty"`
	Token                    string     `json:"token,omitempty"`
	DeviceID                 string     `json:"device_id,omitempty"`
	InitialDeviceDisplayName string     `json:"initial_device_display_name,omitempty"`
}

// ReqCreateRoom is the JSON request for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3createroom
type ReqCreateRoom struct {
	CreationContent   map[string]interface{} `json:"creation_content,omitempty"`
	InitialState      []Event                `json:"initial_state,omitempty"`
	Invite            []string               `json:"invite,omitempty"`
	Invite3PID        []ReqInvite3PID        `json:"invite_3pid,omitempty"`
	IsDirect          bool                   `json:"is_direct,omitempty"`
	Name              string                 `json:"name,omitempty"`
	PowerLevelContent []Event                `json:"power_level_content_override,omitempty"`
	Preset            string                 `json:"preset,omitempty"`
	RoomAliasName     string                 `json:"room_alias_name,omitempty"`
	RoomVersion       string                 `json:"room_version"`
	Topic             string                 `json:"topic,omitempty"`
	Visibility        string                 `json:"visibility,omitempty"`
}

// ReqRedact is the JSON request for https://spec.matrix.org/v1.1/client-server-api/#put_matrixclientv3roomsroomidredacteventidtxnid
type ReqRedact struct {
	Reason string `json:"reason,omitempty"`
}

// ReqInvite3PID is the JSON request for https://matrix.org/docs/spec/client_server/r0.2.0.html#id57
// It is also a JSON object used in https://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-client-r0-createroom
type ReqInvite3PID struct {
	IDServer string `json:"id_server"`
	Medium   string `json:"medium"`
	Address  string `json:"address"`
}

// ReqInviteUser is the JSON request for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidinvite
type ReqInviteUser struct {
	Reason string `json:"reason,omitempty"`
	UserID string `json:"user_id"`
}

// ReqKnockRoom is the JSON request for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3knockroomidoralias
type ReqKnockRoom struct {
	Reason string `json:"reason,omitempty"`
}

// ReqKnockRoom is the JSON request for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidleave
type ReqLeaveRoom struct {
	Reason string `json:"reason,omitempty"`
}

// ReqKickUser is the JSON request for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidkick
type ReqKickUser struct {
	Reason string `json:"reason,omitempty"`
	UserID string `json:"user_id"`
}

// ReqBanUser is the JSON request for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidban
type ReqBanUser struct {
	Reason string `json:"reason,omitempty"`
	UserID string `json:"user_id"`
}

// ReqUnbanUser is the JSON request for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidunban
type ReqUnbanUser struct {
	Reason string `json:"reason,omitempty"`
	UserID string `json:"user_id"`
}

// ReqSetRoomDir is the JSON request for https://spec.matrix.org/v1.1/client-server-api/#put_matrixclientv3directorylistroomroomid
type ReqSetRoomDir struct {
	Visibility string `json:"visibility"`
}

// ReqTyping is the JSON request for https://spec.matrix.org/v1.1/client-server-api/#put_matrixclientv3roomsroomidtypinguserid
type ReqTyping struct {
	Timeout int64 `json:"timeout"`
	Typing  bool  `json:"typing"`
}

// ReqPublicRoomsFiltered is the JSON request for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3publicrooms
type ReqPublicRoomsFiltered struct {
	Filter struct {
		GenericSearchTerm string `json:"generic_search_term"`
	} `json:"filter,omitempty"`
	IncludeAllNetworks   bool   `json:"include_all_networks,omitempty"`
	Limit                int    `json:"limit,omitempty"`
	Since                string `json:"since,omitempty"`
	ThirdPartyInstanceID string `json:"third_party_instance_id"`
}

type ReqSearchUsers struct {
	Limit      int    `json:"limit,omitempty"`
	SearchTerm string `json:"search_term"`
}

type ReqSetProfile struct {
	AvatarUrl string `json:"avatar_url"`
}
