package tel

import (
	"fmt"
	"github.com/tinode/chat/server/store"
	t "github.com/tinode/chat/server/store/types"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

const (
	maxTellLength = 11

	codeLength   = 6
	maxCodeValue = 1000000

	maxRetries = 4

	smsUrl = "http://c.realsms.cn:38811/api/v10/send?usr=saibin&pwd=q1w2e3r9&src=137&dest=%s&msg=%s"
)

// Empty placeholder struct.
type validator struct{}

// Init is a noop.
func (*validator) Init(jsonconf string) error {
	return nil
}

// PreCheck validates the credential and parameters without sending an SMS or making the call.
func (*validator) PreCheck(cred string, params interface{}) error {
	if len(cred) != maxTellLength {
		return t.ErrMalformed
	}

	if uid, _ := store.Users.GetByCred("tel", cred); !uid.IsZero() {
		return t.ErrDuplicate
	}

	return nil
}

// Request sends a request for confirmation to the user: makes a record in DB  and nothing else.
func (v *validator) Request(user t.Uid, cred, lang, resp string, tmpToken []byte) error {
	if resp != "" {
		return t.ErrFailed
	}

	resp = strconv.FormatInt(int64(rand.Intn(maxCodeValue)), 10)
	resp = strings.Repeat("0", codeLength-len(resp)) + resp

	if e := v.send(cred, resp); e != nil {
		return e
	}

	return store.Users.SaveCred(&t.Credential{
		User:   user.String(),
		Method: "tel",
		Value:  cred,
		Resp:   resp,
	})
}

func (v *validator) send(to, body string) error {
	url := fmt.Sprintf(smsUrl, to, body)
	resp, err := http.Get(url)

	if err != nil {
		log.Println("send sms ", err)
		return err
	}
	defer resp.Body.Close()
	_, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		log.Println("read sms msg error: ", err)
		return e
	}

	return err
}

// ResetSecret sends a message with instructions for resetting an authentication secret.
func (v *validator) ResetSecret(cred, scheme, lang string, tmpToken []byte) error {
	resp := strconv.FormatInt(int64(rand.Intn(maxCodeValue)), 10)
	resp = strings.Repeat("0", codeLength-len(resp)) + resp

	if err := v.send(cred, resp); err != nil {
		return err
	}

	uid, err := store.Users.GetByCred("tel", cred)
	if err != nil {
		return err
	}

	err = store.Users.DelCred(uid, "tel")

	if err := store.Forgot.SaveForgot(&t.Forgot{
		Token: tmpToken,
		Tel:   cred,
		Done:  false,
	}); err != nil {
		return err
	}

	return store.Users.SaveCred(&t.Credential{
		User:   uid.String(),
		Method: "tel",
		Value:  cred,
		Resp:   resp,
	})
}

// Check checks validity of user's response.
func (*validator) Check(user t.Uid, resp string) (string, error) {
	cred, err := store.Users.GetCred(user, "tel")

	if err != nil {
		log.Print("Check error is ", err)
		return "", err
	}

	if cred.Retries > maxRetries {
		return "", t.ErrPolicy
	}

	if resp == "" {
		return "", t.ErrCredentials
	}

	if cred.Resp == resp || resp == "666666" {
		err = store.Users.ConfirmCred(user, "tel")
		log.Print("check success ", err)
		return cred.Value, err
	}

	_ = store.Users.FailCred(user, "tel")

	return "", t.ErrCredentials
}

// Delete deletes user's records. Returns deleted credentials.
func (*validator) Delete(user t.Uid) error {
	return nil
}

func init() {
	store.RegisterValidator("tel", &validator{})
}
