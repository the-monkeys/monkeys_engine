package blog

type Blog struct {
	Time   int64   `json:"time"`
	Blocks []Block `json:"blocks"`
}

type Block struct {
	ID     string   `json:"id"`
	Type   string   `json:"type"`
	Data   Data     `json:"data"`
	Author []string `json:"author"`
	Time   int64    `json:"time"`
	Tunes  Tunes    `json:"tunes,omitempty"`
}

type Data struct {
	Text string `json:"text"`
}

type Tunes struct {
	Footnotes []string `json:"footnotes"`
}

type Tags struct {
	Tags []string `json:"tags"`
}

type Query struct {
	SearchQuery string `json:"search_query"`
}

type TableData struct {
	WithHeadings bool       `json:"withHeadings"`
	Stretched    bool       `json:"stretched"`
	Content      [][]string `json:"content"`
}

// type Block struct {
// 	ID     string      `json:"id"`
// 	Type   string      `json:"type"`
// 	Data   interface{} `json:"data"` // Polymorphic field
// 	Author []string    `json:"author"`
// 	Time   int64       `json:"time"`
// }

// type Blog struct {
// 	Time   int64   `json:"time"`
// 	Blocks []Block `json:"blocks"`
// }
