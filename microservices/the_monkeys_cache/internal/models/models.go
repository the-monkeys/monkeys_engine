package models

type TheMonkeysMessage struct {
	Id           int64  `json:"id"`
	AccountId    string `json:"account_id"`
	Username     string `json:"username"`
	NewUsername  string `json:"new_username"`
	Email        string `json:"email"`
	ClientId     string `json:"client_id"`
	Client       string `json:"client"`
	IpAddress    string `json:"ip_address"`
	Action       string `json:"action"`
	BlogId       string `json:"blog_id"`
	BlogStatus   string `json:"blog_status"`
	Notification string `json:"notification"`
}
