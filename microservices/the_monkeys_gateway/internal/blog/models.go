package blog

import "time"

type Tags struct {
	Tags []string `json:"tags"`
}

type PublishBlogReq struct {
	Tags []string `json:"tags"`
	Slug string   `json:"slug"`
}

type ScheduleBlogReq struct {
	PublishBlogReq
	ScheduleTime time.Time `json:"schedule_time"`
	Timezone     string    `json:"timezone"`
}

type Query struct {
	SearchQuery string `json:"search_query"`
}
