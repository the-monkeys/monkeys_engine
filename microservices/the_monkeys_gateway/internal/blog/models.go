package blog

type Tags struct {
	Tags []string `json:"tags"`
}

type PublishBlogReq struct {
	Tags []string `json:"tags"`
	Slug string   `json:"slug"`
}

type Query struct {
	SearchQuery string `json:"search_query"`
}
