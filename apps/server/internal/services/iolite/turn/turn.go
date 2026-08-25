package turn

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"time"
)

func GenerateTurnCredential(userID string, secret string) (string, string) {
	expire := time.Now().Add(1 * time.Hour).Unix()
	username := fmt.Sprintf("%d:%s", expire, userID)

	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(username))
	password := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return username, password
}
