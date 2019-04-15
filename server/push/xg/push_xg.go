package xg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FrontMage/xinge/auth"
	"github.com/tinode/chat/server/push"
	"github.com/tinode/chat/server/store"
	t "github.com/tinode/chat/server/store/types"
	"io/ioutil"
	"log"
	"net/http"
)

var handler XgPush

// How much to buffer the input channel.
const defaultBuffer = 32

type XgPush struct {
	initialized bool
	input       chan *push.Receipt
	stop        chan bool
	auther      *auth.Auther
}

type xgParams struct {
	AudienceType string   `json:"audience_type,omitempty"`
	Platform     string   `json:"platform,omitempty"`
	Message      *Message `json:"message,omitempty"`
	MessageType  string   `json:"message_type,omitempty"`
	AccountList  []string `json:"account_list,omitempty"`
	Environment  string   `json:"environment,omitempty"`
}

type Message struct {
	Title      string   `json:"title,omitempty"`
	Content    string   `json:"content,omitempty"`
	AcceptTime []string `json:"accept_time,omitempty"`

	Android *AndroidParams `json:"android,omitempty"`

	IOS *IOSParams `json:"ios,omitempty"`
}

// AndroidParams 安卓push参数
type AndroidParams struct {
	NID           int                    `json:"n_id,omitempty"`
	BuilderID     int                    `json:"builder_id,omitempty"`
	Ring          int                    `json:"ring,omitempty"`
	RingRaw       string                 `json:"ring_raw,omitempty"`
	Vibrate       int                    `json:"vibrate,omitempty"`
	Lights        int                    `json:"lights,omitempty"`
	Clearable     int                    `json:"clearable,omitempty"`
	IconType      int                    `json:"icon_type,omitempty"`
	IconRes       string                 `json:"icon_res,omitempty"`
	StyleID       int                    `json:"style_id,omitempty"`
	SmallIcon     int                    `json:"small_icon,omitempty"`
	Action        map[string]interface{} `json:"action,omitempty"`
	CustomContent map[string]string      `json:"custom_content,omitempty"`
}

// IOSParams iOS push参数
type IOSParams struct {
	Aps    *Aps              `json:"aps,omitempty"`
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// Aps 通知栏iOS消息的aps字段，详情请参照苹果文档
type Aps struct {
	Alert            map[string]string `json:"alert,omitempty"`
	Badge            int               `json:"badge_type,omitempty"`
	Category         string            `json:"category,omitempty"`
	ContentAvailable int               `json:"content-available,omitempty"`
	Sound            string            `json:"sound,omitempty"`
}

type configType struct {
	Enabled bool `json:"enabled"`
	Buffer  int  `json:"buffer"`
}

// Init initializes the handler
func (XgPush) Init(jsonconf string) error {

	// Check if the handler is already initialized
	if handler.initialized {
		return errors.New("already initialized")
	}

	var config configType
	if err := json.Unmarshal([]byte(jsonconf), &config); err != nil {
		return errors.New("failed to parse config: " + err.Error())
	}

	handler.initialized = true

	if !config.Enabled {
		return nil
	}

	if config.Buffer <= 0 {
		config.Buffer = defaultBuffer
	}

	handler.auther = &auth.Auther{AppID: "dafad9c3e5de3", SecretKey: "4ae0cf1a0d2159b1cda8b3ae347885cc"}

	handler.input = make(chan *push.Receipt, config.Buffer)
	handler.stop = make(chan bool, 1)

	go func() {
		for {
			select {
			case msg := <-handler.input:
				sendNotifications(msg)
			case <-handler.stop:
				return
			}
		}
	}()

	return nil
}

func sendNotifications(rcpt *push.Receipt) {
	uids := make([]t.Uid, len(rcpt.To))
	skipDevices := make(map[string]bool)
	for i, to := range rcpt.To {
		uids[i] = to.User
		// Some devices were online and received the message. Skip them.
		for _, deviceID := range to.Devices {
			skipDevices[deviceID] = true
		}
	}

	devices, count, err := store.Devices.GetAll(uids...)
	if err != nil {
		log.Println("xg push: db err", err)
		return
	}

	if count == 0 {
		return
	}

	for _, devList := range devices {
		for i := range devList {
			d := &devList[i]
			if _, ok := skipDevices[d.DeviceId]; !ok && d.DeviceId != "" {
				switch d.Platform {
				case "ios":
					pushIos(d.DeviceId, &rcpt.Payload2)
				case "android":
				}
			}
		}
	}

}

func pushAndroid(account string) {

}

func pushIos(account string, pl *push.Payload2) {
	url := "https://openapi.xg.qq.com/v3/push/app"

	switch pl.Type {
	case push.PayloadMessage:
		pl.Params["action"] = "message"
	case push.PayloadContact:
		pl.Params["action"] = "contact"
	case push.PayloadSignal:
		pl.Params["action"] = "signal"
	}

	params := xgParams{
		AudienceType: "account",
		Platform:     "ios",
		MessageType:  "notify",
		AccountList:  []string{account},
		Environment:  "dev",
		Message: &Message{
			Title:   pl.Title,
			Content: pl.Content,
			IOS: &IOSParams{
				Aps: &Aps{
					Alert:    map[string]string{"subtitle": ""},
					Badge:    -2,
					Category: "INVITE_CATEGORY",
					Sound:    "Tassel.wav",
				},
				Custom: pl.Params,
			},
		},
	}

	jsonBytes, err := json.Marshal(params)
	if err != nil {
		log.Fatalln("xg push err ", err)
	}
	payload := bytes.NewReader(jsonBytes)
	log.Println("fucking json ", string(jsonBytes))

	req, _ := http.NewRequest("POST", url, payload)
	req.Header.Add("content-type", "application/json")
	req.Header.Add("authorization", "Basic ZGFmYWQ5YzNlNWRlMzo0YWUwY2YxYTBkMjE1OWIxY2RhOGIzYWUzNDc4ODVjYw==")
	req.Header.Add("cache-control", "no-cache")

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	fmt.Println(res)
	fmt.Println(string(body))
}

// IsReady checks if the handler is initialized.
func (XgPush) IsReady() bool {
	return handler.input != nil
}

// Push return a channel that the server will use to send messages to.
// If the adapter blocks, the message will be dropped.
func (XgPush) Push() chan<- *push.Receipt {
	return handler.input
}

// Stop terminates the handler's worker and stops sending pushes.
func (XgPush) Stop() {
	handler.stop <- true
}

func init() {
	push.Register("xg", &handler)
}
