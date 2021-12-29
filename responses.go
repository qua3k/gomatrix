package gomatrix

// RespError is the standard JSON error response from Homeservers. It also implements the Golang "error" interface.
// See http://matrix.org/docs/spec/client_server/r0.2.0.html#api-standards
type RespError struct {
	ErrCode string `json:"errcode"`
	Err     string `json:"error"`
}

// Error returns the errcode and error message.
func (e RespError) Error() string {
	return e.ErrCode + ": " + e.Err
}

// RespCreateFilter is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3useruseridfilter
type RespCreateFilter struct {
	FilterID string `json:"filter_id"`
}

// RespPublicRooms is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3directorylistroomroomid
type RespPublicRooms struct {
	Chunk                  []PublicRoom `json:"chunk"`
	NextBatch              string       `json:"next_batch,omitempty"`
	PrevBatch              string       `json:"prev_batch,omitempty"`
	TotalRoomCountEstimate int          `json:"total_room_count_estimate,omitempty"`
}

// RespJoinRoom is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidjoin
type RespJoinRoom struct {
	RoomID string `json:"room_id"`
}

// RespResolveRoomAlias is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3directoryroomroomalias
type RespResolveRoomAlias struct {
	RoomID  string   `json:"room_id"`
	Servers []string `json:"servers"`
}

// RespLeaveRoom is the JSON response for http://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-client-r0-rooms-roomid-leave
type RespLeaveRoom struct{}

// RespForgetRoom is the JSON response for http://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-client-r0-rooms-roomid-forget
type RespForgetRoom struct{}

// RespServerCapabilities is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3capabilities
type RespServerCapabilities struct {
	Capabilities map[string]interface{} `json:"capabilities"`
}

// RespInviteUser is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidinvite
type RespInviteUser struct{}

// RespKnockRoom is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3knockroomidoralias
type RespKnockRoom struct {
	RoomID string `json:"room_id"`
}

// RespKickUser is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidkick
type RespKickUser struct{}

// RespBanUser is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidban
type RespBanUser struct{}

// RespUnbanUser is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidunban
type RespUnbanUser struct{}

// RespGetRoomDir is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3directorylistroomroomid
type RespGetRoomDir struct {
	Visibility string `json:"visibility"`
}

// RespSetRoomDir is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#put_matrixclientv3directorylistroomroomid
type RespSetRoomDir struct{}

// RespJoinedRooms is the JSON response for TODO-SPEC https://github.com/matrix-org/synapse/pull/1680
type RespJoinedRooms struct {
	JoinedRooms []string `json:"joined_rooms"`
}

// RespJoinedMembers is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3roomsroomidjoined_members
type RespJoinedMembers struct {
	Joined map[string]struct {
		AvatarURL   string `json:"avatar_url,omitempty"`
		DisplayName string `json:"display_name,omitempty"`
	} `json:"joined"`
}

// RespMessages is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3roomsroomidmessages
type RespMessages struct {
	Chunk []Event `json:"chunk"`
	End   string  `json:"end"`
	Start string  `json:"start"`
	State []Event `json:"state"`
}

// RespSendEvent is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#put_matrixclientv3roomsroomidsendeventtypetxnid
type RespSendEvent struct {
	EventID string `json:"event_id"`
}

// RespMediaUpload is the JSON response for http://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-media-r0-upload
type RespMediaUpload struct {
	ContentURI string `json:"content_uri"`
}

// RespUserInteractive is the JSON response for https://matrix.org/docs/spec/client_server/r0.2.0.html#user-interactive-authentication-api
type RespUserInteractive struct {
	Flows []struct {
		Stages []string `json:"stages"`
	} `json:"flows"`
	Params    map[string]interface{} `json:"params"`
	Session   string                 `json:"session"`
	Completed []string               `json:"completed"`
	ErrCode   string                 `json:"errcode"`
	Error     string                 `json:"error"`
}

// HasSingleStageFlow returns true if there exists at least 1 Flow with a single stage of stageName.
func (r RespUserInteractive) HasSingleStageFlow(stageName string) bool {
	for _, f := range r.Flows {
		if len(f.Stages) == 1 && f.Stages[0] == stageName {
			return true
		}
	}
	return false
}

// RespRegister is the JSON response for http://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-client-r0-register
type RespRegister struct {
	AccessToken  string `json:"access_token"`
	DeviceID     string `json:"device_id"`
	HomeServer   string `json:"home_server"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
}

// RespLogin is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3login
type RespLogin struct {
	AccessToken string               `json:"access_token"`
	DeviceID    string               `json:"device_id"`
	UserID      string               `json:"user_id"`
	WellKnown   DiscoveryInformation `json:"well_known"`
}

// DiscoveryInformation is the JSON Response for https://spec.matrix.org/latest/client-server-api/#getwell-knownmatrixclient and a part of the JSON Response for https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3login
type DiscoveryInformation struct {
	Homeserver struct {
		BaseURL string `json:"base_url"`
	} `json:"m.homeserver"`
	IdentityServer struct {
		BaseURL string `json:"base_url"`
	} `json:"m.identitiy_server"`
}

// RespCreateRoom is the JSON response for https://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-client-r0-createroom
type RespCreateRoom struct {
	RoomID string `json:"room_id"`
}

// The timeline object
type Timeline struct {
	Events    []Event `json:"events"`
	Limited   bool    `json:"limited"`
	PrevBatch string  `json:"prev_batch,omitempty"`
}

// The join object
type Join struct {
	AccountData struct {
		Events []Event `json:"events"`
	} `json:"account_data"`
	Ephemeral struct {
		Events []Event `json:"events"`
	} `json:"ephemeral"`
	State struct {
		Events []Event `json:"events"`
	} `json:"state"`
	Summary struct {
		Heros              []string `json:"m.heros,omitempty"`
		InvitedMemberCount int      `json:"m.invited_member_count,omitempty"`
		JoinedMemberCount  int      `json:"m.joined_member_count,omitempty"`
	} `json:"summary"`
	Timeline            Timeline `json:"timeline"`
	UnreadNotifications struct {
		HighLightCount    int `json:"highlight_count"`
		NotificationCount int `json:"notification_count"`
	} `json:"unread_notifications"`
}

// RespSync is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3sync
type RespSync struct {
	AccountData struct {
		Events []Event `json:"events"`
	} `json:"account_data"`
	NextBatch string `json:"next_batch"`
	Presence  struct {
		Events []Event `json:"events"`
	} `json:"presence"`
	Rooms struct {
		Invite map[string]struct {
			State struct {
				Events []Event `json:"events"`
			} `json:"invite_state"`
		} `json:"invite"`
		Join  map[string]Join `json:"join"`
		Knock map[string]struct {
			State struct {
				Events []Event `json:"events"`
			} `json:"knock_state"`
		}
		Leave map[string]struct {
			AccountData struct {
				Events []Event `json:"events"`
			} `json:"account_data"`
			State struct {
				Events []Event `json:"events"`
			} `json:"state"`
			Timeline Timeline `json:"timeline"`
		} `json:"leave"`
	} `json:"rooms"`
	ToDevice struct {
		Events []Event `json:"events"`
	} `json:"to_device"`
}

type RespSearchUsers struct {
	Limited bool `json:"limited"`
	Results []struct {
		AvatarURL   string `json:"avatar_url,omitempty"`
		DisplayName string `json:"display_name,omitempty"`
		UserId      string `json:"user_id"`
	} `json:"results"`
}

// RespPowerLevels is the JSON response for https://spec.matrix.org/v1.1/client-server-api/#mroompower_levels
type RespPowerLevels struct {
	Ban           int            `json:"ban,omitempty"`
	Events        map[string]int `json:"events,omitempty"`
	EventsDefault int            `json:"events_default,omitempty"`
	Invite        int            `json:"invite,omitempty"`
	Kick          int            `json:"kick,omitempty"`
	Notifications struct {
		Room int `json:"room,omitempty"`
	} `json:"notifications"`
	Redact       int            `json:"redact,omitempty"`
	StateDefault int            `json:"state_default,omitempty"`
	Users        map[string]int `json:"users"`
	UsersDefault int            `json:"users_default,omitempty"`
	Room         int            `json:"room,omitempty"`
}
