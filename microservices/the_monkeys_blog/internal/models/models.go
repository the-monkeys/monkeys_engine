package models

// End of the struct

type Blog struct {
	Time   int64   `json:"time"`
	Blocks []Block `json:"blocks"`
}

type Block struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Data   Data   `json:"data"`
	Author string `json:"author"`
	Time   int64  `json:"time"`
	Tunes  Tunes  `json:"tunes,omitempty"`
}

type Data struct {
	Text string `json:"text"`
}

type Tunes struct {
	Footnotes []string `json:"footnotes"`
}

type MessageToUserSvc struct {
	UserAccountId string `json:"user_account_id"`
	BlogId        string `json:"blog_id"`
	Action        string `json:"action"`
	Status        string `json:"status"`
}
