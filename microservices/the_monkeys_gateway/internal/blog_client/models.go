package blog_client

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
