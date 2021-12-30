// Package gomatrix implements the Matrix Client-Server API.
//
// Specification can be found at https://spec.matrix.org/v1.1/client-server-api/
package gomatrix

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client represents a Matrix client.
type Client struct {
	HomeserverURL *url.URL     // The base homeserver URL
	Prefix        string       // The API prefix eg '/_matrix/client/v3'
	UserID        string       // The user ID of the client. Used for forming HTTP paths which use the client's user ID.
	AccessToken   string       // The access_token for the client.
	Client        *http.Client // The underlying HTTP client which will be used to make HTTP requests.
	Syncer        Syncer       // The thing which can process /sync responses
	Store         Storer       // The thing which can store rooms/tokens/ids

	// The ?user_id= query parameter for application services. This must be set *prior* to calling a method. If this is empty,
	// no user_id parameter will be sent.
	// See http://matrix.org/docs/spec/application_service/unstable.html#identity-assertion
	AppServiceUserID string

	syncingMutex sync.Mutex // protects syncingID
	syncingID    uint32     // Identifies the current Sync. Only one Sync can be active at any given time.
}

// HTTPError An HTTP Error response, which may wrap an underlying native Go Error.
type HTTPError struct {
	Contents     []byte
	WrappedError error
	Message      string
	Code         int
}

func (e HTTPError) Error() string {
	var wrappedErrMsg string
	if e.WrappedError != nil {
		wrappedErrMsg = e.WrappedError.Error()
	}
	return fmt.Sprintf("contents=%v msg=%s code=%d wrapped=%s", e.Contents, e.Message, e.Code, wrappedErrMsg)
}

// BuildURL builds a URL with the Client's homeserver/prefix set already.
func (cli *Client) BuildURL(urlPath ...string) string {
	ps := append([]string{cli.Prefix}, urlPath...)
	return cli.BuildBaseURL(ps...)
}

// BuildBaseURL builds a URL with the Client's homeserver set already. You must
// supply the prefix in the path.
func (cli *Client) BuildBaseURL(urlPath ...string) string {
	// copy the URL. Purposefully ignore error as the input is from a valid URL already
	hsURL, _ := url.Parse(cli.HomeserverURL.String())
	parts := []string{hsURL.Path}
	parts = append(parts, urlPath...)
	hsURL.Path = path.Join(parts...)
	// Manually add the trailing slash back to the end of the path if it's explicitly needed
	if strings.HasSuffix(urlPath[len(urlPath)-1], "/") {
		hsURL.Path = hsURL.Path + "/"
	}
	query := hsURL.Query()
	if cli.AppServiceUserID != "" {
		query.Set("user_id", cli.AppServiceUserID)
	}
	hsURL.RawQuery = query.Encode()
	return hsURL.String()
}

// BuildURLWithQuery builds a URL with query parameters in addition to the Client's homeserver/prefix set already.
func (cli *Client) BuildURLWithQuery(urlPath []string, urlQuery map[string]string) string {
	u, _ := url.Parse(cli.BuildURL(urlPath...))
	q := u.Query()
	for k, v := range urlQuery {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// SetCredentials sets the user ID and access token on this client instance.
func (cli *Client) SetCredentials(userID, accessToken string) {
	cli.AccessToken = accessToken
	cli.UserID = userID
}

// ClearCredentials removes the user ID and access token on this client instance.
func (cli *Client) ClearCredentials() {
	cli.AccessToken = ""
	cli.UserID = ""
}

// Sync starts syncing with the provided Homeserver. If Sync() is called twice then the first sync will be stopped and the
// error will be nil.
//
// This function will block until a fatal /sync error occurs, so it should almost always be started as a new goroutine.
// Fatal sync errors can be caused by:
//   - The failure to create a filter.
//   - Client.Syncer.OnFailedSync returning an error in response to a failed sync.
//   - Client.Syncer.ProcessResponse returning an error.
// If you wish to continue retrying in spite of these fatal errors, call Sync() again.
func (cli *Client) Sync() error {
	// Mark the client as syncing.
	// We will keep syncing until the syncing state changes. Either because
	// Sync is called or StopSync is called.
	syncingID := cli.incrementSyncingID()
	nextBatch := cli.Store.LoadNextBatch(cli.UserID)
	filterID := cli.Store.LoadFilterID(cli.UserID)
	if filterID == "" {
		filterJSON := cli.Syncer.GetFilterJSON(cli.UserID)
		resFilter, err := cli.CreateFilter(filterJSON)
		if err != nil {
			return err
		}
		filterID = resFilter.FilterID
		cli.Store.SaveFilterID(cli.UserID, filterID)
	}

	for {
		resSync, err := cli.SyncRequest(30000, nextBatch, filterID, false, "")
		if err != nil {
			duration, err2 := cli.Syncer.OnFailedSync(resSync, err)
			if err2 != nil {
				return err2
			}
			time.Sleep(duration)
			continue
		}

		// Check that the syncing state hasn't changed
		// Either because we've stopped syncing or another sync has been started.
		// We discard the response from our sync.
		if cli.getSyncingID() != syncingID {
			return nil
		}

		// Save the token now *before* processing it. This means it's possible
		// to not process some events, but it means that we won't get constantly stuck processing
		// a malformed/buggy event which keeps making us panic.
		cli.Store.SaveNextBatch(cli.UserID, resSync.NextBatch)
		if err = cli.Syncer.ProcessResponse(resSync, nextBatch); err != nil {
			return err
		}

		nextBatch = resSync.NextBatch
	}
}

func (cli *Client) incrementSyncingID() uint32 {
	cli.syncingMutex.Lock()
	defer cli.syncingMutex.Unlock()
	cli.syncingID++
	return cli.syncingID
}

func (cli *Client) getSyncingID() uint32 {
	cli.syncingMutex.Lock()
	defer cli.syncingMutex.Unlock()
	return cli.syncingID
}

// StopSync stops the ongoing sync started by Sync.
func (cli *Client) StopSync() {
	// Advance the syncing state so that any running Syncs will terminate.
	cli.incrementSyncingID()
}

// MakeRequest makes a JSON HTTP request to the given URL.
// The response body will be stream decoded into an interface. This will automatically stop if the response
// body is nil.
//
// Returns an error if the response is not 2xx along with the HTTP body bytes if it got that far. This error is
// an HTTPError which includes the returned HTTP status code, byte contents of the response body and possibly a
// RespError as the WrappedError, if the HTTP body could be decoded as a RespError.
func (cli *Client) MakeRequest(method string, httpURL string, reqBody interface{}, resBody interface{}) error {
	var (
		req   *http.Request
		err   error
		sleep time.Duration = 5000000000 // 5 seconds
	)
	if reqBody != nil {
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(reqBody); err != nil {
			return err
		}
		req, err = http.NewRequest(method, httpURL, buf)
	} else {
		req, err = http.NewRequest(method, httpURL, nil)
	}

	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	if cli.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+cli.AccessToken)
	}

	res, err := cli.Client.Do(req)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return err
	}
	if res.StatusCode == 429 {
		dur, err := HandleRetry(res, sleep)
		if err != nil {
			return err
		}
		time.Sleep(dur)
		cli.MakeRequest(method, httpURL, reqBody, resBody)
	} else if res.StatusCode/100 != 2 { // not 2xx
		contents, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}

		var wrap error
		var respErr RespError
		if _ = json.Unmarshal(contents, &respErr); respErr.ErrCode != "" {
			wrap = respErr
		}

		// If we failed to decode as RespError, don't just drop the HTTP body, include it in the
		// HTTP error instead (e.g proxy errors which return HTML).
		msg := "Failed to " + method + " JSON to " + req.URL.Path
		if wrap == nil {
			msg = msg + ": " + string(contents)
		}

		return HTTPError{
			Contents:     contents,
			Code:         res.StatusCode,
			Message:      msg,
			WrappedError: wrap,
		}
	}

	if resBody != nil && res.Body != nil {
		return json.NewDecoder(res.Body).Decode(&resBody)
	}

	return nil
}

func HandleRetry(res *http.Response, duration time.Duration) (time.Duration, error) {
	ra := res.Header.Get("Retry-After")
	if ra == "" {
		return duration, nil
	}

	if t, err := time.Parse(http.TimeFormat, ra); err == nil {
		return time.Until(t), nil
	}

	if seconds, err := strconv.Atoi(ra); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}

	return duration, errors.New("invalid retry-after data")
}

// CreateFilter makes an HTTP request according to https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3useruseridfilter
func (cli *Client) CreateFilter(filter json.RawMessage) (resp *RespCreateFilter, err error) {
	urlPath := cli.BuildURL("user", cli.UserID, "filter")
	err = cli.MakeRequest("POST", urlPath, &filter, &resp)
	return
}

// GetFilter makes an HTTP request according to https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3useruseridfilterfilterid
func (cli *Client) GetFilter(filterID string) (resp *Filter, err error) {
	urlPath := cli.BuildURL("user", cli.UserID, "filter", filterID)
	err = cli.MakeRequest("GET", urlPath, nil, &resp)
	return
}

// SyncRequest makes an HTTP request according to https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3sync
func (cli *Client) SyncRequest(timeout int, since, filterID string, fullState bool, setPresence string) (resp *RespSync, err error) {
	query := map[string]string{
		"timeout": strconv.Itoa(timeout),
	}
	if filterID != "" {
		query["filter"] = filterID
	}
	if fullState {
		query["full_state"] = "true"
	}
	if setPresence != "" {
		query["set_presence"] = setPresence
	}
	if since != "" {
		query["since"] = since
	}
	urlPath := cli.BuildURLWithQuery([]string{"sync"}, query)
	err = cli.MakeRequest("GET", urlPath, nil, &resp)
	return
}

// GetEventByID returns a single event based on roomId/eventId. See https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3roomsroomideventeventid
func (cli *Client) GetEventByID(eventID, roomID string) (resp *Event, err error) {
	u := cli.BuildURL("rooms", roomID, "event", eventID)
	err = cli.MakeRequest("GET", u, nil, &resp)
	return
}

// JoinedMembers returns a map of joined room members. See https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3roomsroomidjoined_members
//
// This API is primarily for Application Services and should be faster to
// respond than /members as it can be implemented more efficiently on the server.
func (cli *Client) JoinedMembers(roomID string) (resp *RespJoinedMembers, err error) {
	u := cli.BuildURL("rooms", roomID, "joined_members")
	err = cli.MakeRequest("GET", u, nil, &resp)
	return
}

// GetMembers returns the list of members for a room. See https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3roomsroomidmembers
func (cli *Client) GetMembers(at, membership, notMembership, roomID string) (resp *RespJoinedMembers, err error) {
	query := map[string]string{}

	if at != "" {
		query["at"] = at
	}
	if membership != "" {
		query["membership"] = membership
	}
	if notMembership != "" {
		query["not_membership"] = notMembership
	}

	urlPath := cli.BuildURLWithQuery([]string{"rooms", roomID, "members"}, query)
	err = cli.MakeRequest("GET", urlPath, nil, &resp)
	return
}

// GetStateEvent returns the state events for the current state of the room. See https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3roomsroomidstate
func (cli *Client) GetStateEvents(roomID string) (resp *Event, err error) {
	urlPath := cli.BuildURL("rooms", roomID, "state")
	err = cli.MakeRequest("GET", urlPath, nil, &resp)
	return
}

// PowerLevels returns the power levels content for the current state of the room. See https://spec.matrix.org/v1.1/client-server-api/#mroompower_levels
func (cli *Client) PowerLevels(roomID string) (resp *RespPowerLevels, err error) {
	err = cli.StateEvent(roomID, "m.room.power_levels", "", &resp)
	return
}

// Context returns a number of events that happened just before and after the
// specified event. It use pagination query parameters to paginate history in
// the room.
// See https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3roomsroomidcontexteventid
func (cli *Client) Context(roomID, eventID, filter string, limit int) (resp *RespContext, err error) {
	query := map[string]string{}

	if filter != "" {
		query["filter"] = filter
	}

	if limit != 0 {
		query["limit"] = strconv.Itoa(limit)
	}

	urlPath := cli.BuildURLWithQuery([]string{"rooms", roomID, "context", eventID}, query)
	err = cli.MakeRequest("GET", urlPath, nil, &resp)
	return
}

// Messages returns a list of message and state events for a room. It uses
// pagination query parameters to paginate history in the room.
// See https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3roomsroomidmessages
func (cli *Client) Messages(roomID, filter, from, to string, dir rune, limit int) (resp *RespMessages, err error) {
	query := map[string]string{
		"dir": string(dir),
	}

	if from != "" {
		query["from"] = from // this is not spec compliant, see https://github.com/matrix-org/matrix-doc/pull/3567
	}

	if filter != "" {
		query["filter"] = filter
	}

	if limit != 0 {
		query["limit"] = strconv.Itoa(limit)
	}

	if to != "" {
		query["to"] = to
	}

	urlPath := cli.BuildURLWithQuery([]string{"rooms", roomID, "messages"}, query)
	err = cli.MakeRequest("GET", urlPath, nil, &resp)
	return
}

// SendStateEvent sends a state event into a room. See https://spec.matrix.org/v1.1/client-server-api/#put_matrixclientv3roomsroomidstateeventtypestatekey
// contentJSON should be a pointer to something that can be encoded as JSON using json.Marshal.
func (cli *Client) SendStateEvent(roomID, eventType, stateKey string, contentJSON interface{}) (resp *RespSendEvent, err error) {
	urlPath := cli.BuildURL("rooms", roomID, "state", eventType, stateKey)
	err = cli.MakeRequest("PUT", urlPath, contentJSON, &resp)
	return
}

// SendMessageEvent sends a message event into a room. See https://spec.matrix.org/v1.1/client-server-api/#put_matrixclientv3roomsroomidsendeventtypetxnid
// contentJSON should be a pointer to something that can be encoded as JSON using json.Marshal.
func (cli *Client) SendMessageEvent(roomID, eventType string, contentJSON interface{}) (resp *RespSendEvent, err error) {
	txnID := txnID()
	urlPath := cli.BuildURL("rooms", roomID, "send", eventType, txnID)
	err = cli.MakeRequest("PUT", urlPath, contentJSON, &resp)
	return
}

// RedactEvent redacts the given event. See https://spec.matrix.org/v1.1/client-server-api/#put_matrixclientv3roomsroomidredacteventidtxnid
func (cli *Client) RedactEvent(roomID, eventID string, req *ReqRedact) (resp *RespSendEvent, err error) {
	txnID := txnID()
	urlPath := cli.BuildURL("rooms", roomID, "redact", eventID, txnID)
	err = cli.MakeRequest("PUT", urlPath, req, &resp)
	return
}

// CreateRoom creates a new Matrix room. See https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3createroom
//  resp, err := cli.CreateRoom(&gomatrix.ReqCreateRoom{
//  	Preset: "public_chat",
//  })
//  fmt.Println("Room:", resp.RoomID)
func (cli *Client) CreateRoom(req *ReqCreateRoom) (resp *RespCreateRoom, err error) {
	urlPath := cli.BuildURL("createRoom")
	err = cli.MakeRequest("POST", urlPath, req, &resp)
	return
}

// JoinedRooms returns a list of rooms which the client is joined to. See https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3joined_rooms
//
// In general, usage of this API is discouraged in favour of /sync, as calling this API can race with incoming membership changes.
// This API is primarily designed for application services which may want to efficiently look up joined rooms.
func (cli *Client) JoinedRooms() (resp *RespJoinedRooms, err error) {
	u := cli.BuildURL("joined_rooms")
	err = cli.MakeRequest("GET", u, nil, &resp)
	return
}

// JoinRoomIDOrAlias joins the client to a room ID or alias. See http://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-client-r0-join-roomidoralias
//
// If serverName is specified, this will be added as a query param to instruct the homeserver to join via that server. If content is specified, it will
// be JSON encoded and used as the request body.
func (cli *Client) JoinRoomIDOrAlias(roomIDorAlias, serverName string, content interface{}) (resp *RespJoinRoom, err error) {
	var urlPath string
	if serverName != "" {
		urlPath = cli.BuildURLWithQuery([]string{"join", roomIDorAlias}, map[string]string{
			"server_name": serverName,
		})
	} else {
		urlPath = cli.BuildURL("join", roomIDorAlias)
	}
	err = cli.MakeRequest("POST", urlPath, content, &resp)
	return
}

// JoinRoom joins the client to a room ID. See https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidjoin
//
// If serverName is specified, this will be added as a query param to instruct the homeserver to join via that server. If content is specified, it will
// be JSON encoded and used as the request body.
func (cli *Client) JoinRoom(roomID, serverName string, content interface{}) (resp *RespJoinRoom, err error) {
	var urlPath string
	if serverName != "" {
		urlPath = cli.BuildURLWithQuery([]string{"rooms", roomID, "join"}, map[string]string{
			"server_name": serverName,
		})
	} else {
		urlPath = cli.BuildURL("rooms", roomID, "join")
	}
	err = cli.MakeRequest("POST", urlPath, content, &resp)
	return
}

// KnockRoom “knocks” on the room to ask for permission to join. See https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3knockroomidoralias
func (cli *Client) KnockRoom(roomIDorAlias, serverName string, req *ReqKnockRoom) (resp *RespKnockRoom, err error) {
	var urlPath string
	if serverName != "" {
		urlPath = cli.BuildURLWithQuery([]string{"knock", roomIDorAlias}, map[string]string{
			"server_name": serverName,
		})
	} else {
		urlPath = cli.BuildURL("knock", roomIDorAlias)
	}
	err = cli.MakeRequest("POST", urlPath, req, &resp)
	return
}

// ForgetRoom forgets a room entirely. See https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidforget
func (cli *Client) ForgetRoom(roomID string) (resp *RespForgetRoom, err error) {
	u := cli.BuildURL("rooms", roomID, "forget")
	err = cli.MakeRequest("POST", u, struct{}{}, &resp)
	return
}

// LeaveRoom leaves the given room. See https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidleave
func (cli *Client) LeaveRoom(roomID string, req *ReqLeaveRoom) (resp *RespLeaveRoom, err error) {
	u := cli.BuildURL("rooms", roomID, "leave")
	err = cli.MakeRequest("POST", u, req, &resp)
	return
}

// KickUser kicks a user from a room. See https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidkick
func (cli *Client) KickUser(roomID string, req *ReqKickUser) (resp *RespKickUser, err error) {
	u := cli.BuildURL("rooms", roomID, "kick")
	err = cli.MakeRequest("POST", u, req, &resp)
	return
}

// BanUser bans a user from a room. See https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidkick
func (cli *Client) BanUser(roomID string, req *ReqBanUser) (resp *RespBanUser, err error) {
	u := cli.BuildURL("rooms", roomID, "ban")
	err = cli.MakeRequest("POST", u, req, &resp)
	return
}

// UnbanUser unbans a user from a room. See https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3roomsroomidunban
func (cli *Client) UnbanUser(roomID string, req *ReqUnbanUser) (resp *RespUnbanUser, err error) {
	u := cli.BuildURL("rooms", roomID, "unban")
	err = cli.MakeRequest("POST", u, req, &resp)
	return
}

// GetRoomDir gets the visibility of a given room on the server’s public room directory. See https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3directorylistroomroomid
func (cli *Client) GetRoomDir(roomID string) (resp *RespGetRoomDir, err error) {
	u := cli.BuildURL("directory", "list", "room", roomID)
	err = cli.MakeRequest("GET", u, nil, &resp)
	return
}

// SetRoomDir gets the visibility of a given room on the server’s public room directory. See https://spec.matrix.org/v1.1/client-server-api/#put_matrixclientv3directorylistroomroomid
func (cli *Client) SetRoomDir(roomID string, req *ReqSetRoomDir) (resp *RespSetRoomDir, err error) {
	u := cli.BuildURL("directory", "list", "room", roomID)
	err = cli.MakeRequest("PUT", u, req, &resp)
	return
}

// SearchUsers performs a search for users on the homeserver. See https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3user_directorysearch
func (cli *Client) SearchUsers(req *ReqSearchUsers) (resp *RespSearchUsers, err error) {
	urlPath := cli.BuildURL("user_directory", "search")
	err = cli.MakeRequest("POST", urlPath, struct{}{}, &resp)
	return
}

// SendText sends an m.room.message event into the given room with a msgtype of m.text
// See http://matrix.org/docs/spec/client_server/r0.2.0.html#m-text
func (cli *Client) SendText(roomID, text string) (*RespSendEvent, error) {
	return cli.SendMessageEvent(roomID, "m.room.message",
		TextMessage{MsgType: "m.text", Body: text})
}

func (cli *Client) register(u string, req *ReqRegister) (resp *RespRegister, uiaResp *RespUserInteractive, err error) {
	err = cli.MakeRequest("POST", u, req, &resp)
	if err != nil {
		httpErr, ok := err.(HTTPError)
		if !ok { // network error
			return
		}
		if httpErr.Code == 401 {
			// body should be RespUserInteractive, if it isn't, fail with the error
			err = json.Unmarshal(httpErr.Contents, &uiaResp)
			return
		}
	}
	return
}

// Register makes an HTTP request according to http://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-client-r0-register
//
// Registers with kind=user. For kind=guest, see RegisterGuest.
func (cli *Client) Register(req *ReqRegister) (*RespRegister, *RespUserInteractive, error) {
	u := cli.BuildURL("register")
	return cli.register(u, req)
}

// Login a user to the homeserver according to https://spec.matrix.org/v1.1/client-server-api/#post_matrixclientv3login
// This does not set credentials on this client instance. See SetCredentials() instead.
func (cli *Client) Login(req *ReqLogin) (resp *RespLogin, err error) {
	urlPath := cli.BuildURL("login")
	err = cli.MakeRequest("POST", urlPath, req, &resp)
	return
}

// SendFormattedText sends an m.room.message event into the given room with a msgtype of m.text, supports a subset of HTML for formatting.
// See https://matrix.org/docs/spec/client_server/r0.6.0#m-text
func (cli *Client) SendFormattedText(roomID, text, formattedText string) (*RespSendEvent, error) {
	return cli.SendMessageEvent(roomID, "m.room.message",
		TextMessage{MsgType: "m.text", Body: text, FormattedBody: formattedText, Format: "org.matrix.custom.html"})
}

// SendSticker sends an m.room.message event into the given room with a msgtype of m.sticker
// See https://spec.matrix.org/latest/client-server-api/#msticker
func (cli *Client) SendSticker(roomID, body, url string) (*RespSendEvent, error) {
	return cli.SendMessageEvent(roomID, "m.sticker",
		ImageMessage{
			Body: body,
			Info: ImageInfo{
				Height: 256,
				ThumbnailInfo: ThumbnailInfo{
					Height: 256,
					Width:  256,
				},
				ThumbnailURL: url,
				Width:        256,
			},
			URL: url,
		})
}

// SendNotice sends an m.room.message event into the given room with a msgtype of m.notice
// See https://spec.matrix.org/v1.1/client-server-api/#mnotice
func (cli *Client) SendNotice(roomID, text string) (*RespSendEvent, error) {
	return cli.SendMessageEvent(roomID, "m.room.message",
		TextMessage{MsgType: "m.notice", Body: text})
}

// InviteUserByThirdParty invites a third-party identifier to a room. See http://matrix.org/docs/spec/client_server/r0.2.0.html#invite-by-third-party-id-endpoint
func (cli *Client) InviteUserByThirdParty(roomID string, req *ReqInvite3PID) (resp *RespInviteUser, err error) {
	u := cli.BuildURL("rooms", roomID, "invite")
	err = cli.MakeRequest("POST", u, req, &resp)
	return
}

// StateEvent gets a single state event in a room. It will attempt to JSON unmarshal into the given "outContent" struct with
// the HTTP response body, or return an error.
// See https://spec.matrix.org/v1.1/client-server-api/#get_matrixclientv3roomsroomidstateeventtypestatekey
func (cli *Client) StateEvent(roomID, eventType, stateKey string, outContent interface{}) (err error) {
	u := cli.BuildURL("rooms", roomID, "state", eventType, stateKey)
	err = cli.MakeRequest("GET", u, nil, outContent)
	return
}

// UploadLink uploads an HTTP URL and then returns an MXC URI.
func (cli *Client) UploadLink(link string) (*RespMediaUpload, error) {
	res, err := cli.Client.Get(link)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	return cli.UploadToContentRepo(res.Body, res.Header.Get("Content-Type"), res.ContentLength)
}

// UploadToContentRepo uploads the given bytes to the content repository and returns an MXC URI.
// See http://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-media-r0-upload
func (cli *Client) UploadToContentRepo(content io.Reader, contentType string, contentLength int64) (*RespMediaUpload, error) {
	req, err := http.NewRequest("POST", cli.BuildBaseURL("_matrix/media/r0/upload"), content)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+cli.AccessToken)

	req.ContentLength = contentLength

	res, err := cli.Client.Do(req)
	if res != nil {
		defer res.Body.Close()
	}

	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		contents, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, HTTPError{
				Message: "Upload request failed - Failed to read response body: " + err.Error(),
				Code:    res.StatusCode,
			}
		}
		return nil, HTTPError{
			Contents: contents,
			Message:  "Upload request failed: " + string(contents),
			Code:     res.StatusCode,
		}
	}

	var m RespMediaUpload
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		return nil, err
	}

	return &m, nil
}

func txnID() string {
	return "go" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

// NewClient creates a new Matrix Client ready for syncing
func NewClient(homeserverURL, userID, accessToken string) (*Client, error) {
	hsURL, err := url.Parse(homeserverURL)
	if err != nil {
		return nil, err
	}
	// By default, use an in-memory store which will never save filter ids / next batch tokens to disk.
	// The client will work with this storer: it just won't remember across restarts.
	// In practice, a database backend should be used.
	store := NewInMemoryStore()
	cli := Client{
		AccessToken:   accessToken,
		HomeserverURL: hsURL,
		UserID:        userID,
		Prefix:        "/_matrix/client/v3",
		Syncer:        NewDefaultSyncer(userID, store),
		Store:         store,
	}
	// By default, use the default HTTP client.
	cli.Client = http.DefaultClient

	return &cli, nil
}
