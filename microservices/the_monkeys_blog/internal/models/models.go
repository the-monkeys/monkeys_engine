package models

import (
	"time"

	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/pb"
)

type Blogs struct {
	Id               string                         `json:"id"`
	Title            string                         `json:"title"`
	ContentFormatted string                         `json:"content_formatted"`
	ContentRaw       string                         `json:"content_raw"`
	AuthorName       string                         `json:"author_name"`
	AuthorId         string                         `json:"author_id"`
	Published        *bool                          `json:"published"`
	Tags             []string                       `json:"tags"`
	CreateTime       string                         `json:"create_time"`
	UpdateTime       string                         `json:"update_time"`
	CanEdit          *bool                          `json:"can_edit"`
	OwnerShip        pb.CreateBlogRequest_Ownership `json:"content_ownership"`
	FolderPath       string                         `json:"folder_path"`
}

type BlogsService struct {
	Id                 string   `json:"id"`
	HTMLContent        string   `json:"html_content"`
	RawContent         string   `json:"raw_content"`
	CreateTime         string   `json:"create_time"`
	UpdateTime         string   `json:"update_time"`
	AuthorName         string   `json:"author_name"`
	AuthorEmail        string   `json:"author_email"`
	AuthorStatus       string   `json:"author_status"`
	Published          *bool    `json:"published"`
	NoOfViews          int      `json:"no_of_views"`
	Tags               []string `json:"tags"`
	CanEdit            *bool    `json:"can_edit"`
	OwnerShip          string   `json:"content_ownership"`
	Category           string   `json:"category"`
	FirstPublishedTime string   `json:"first_published_time"`
	LastEditedTime     string   `json:"last_edited_time"`
}

type GetArticleResp struct {
	Author     string `json:"author"`
	CreateTime string `json:"create_time"`
	ID         string `json:"id"`
	QuickRead  string `json:"quick_read"`
	Title      string `json:"title"`
	ViewedBy   string `json:"viewed_by"`
}

// type Last100Articles struct {
// 	AuthorName string    `json:"author_name"`
// 	ContentRaw string    `json:"content_raw"`
// 	CreateTime time.Time `json:"create_time"`
// 	ID         string    `json:"id"`
// 	Title      string    `json:"title"`
// 	AuthorID   string    `json:"author_id"`
// }

// Needs to be deleted
type Last100Articles struct {
	Took     int  `json:"took"`
	TimedOut bool `json:"timed_out"`
	Shards   struct {
		Total      int `json:"total"`
		Successful int `json:"successful"`
		Skipped    int `json:"skipped"`
		Failed     int `json:"failed"`
	} `json:"_shards"`
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		MaxScore interface{} `json:"max_score"`
		Hits     []struct {
			Index  string      `json:"_index"`
			ID     string      `json:"_id"`
			Score  interface{} `json:"_score"`
			Source struct {
				AuthorName string    `json:"author_name"`
				ContentRaw string    `json:"content_raw"`
				CreateTime time.Time `json:"create_time"`
				ID         string    `json:"id"`
				Title      string    `json:"title"`
				AuthorID   string    `json:"author_id"`
			} `json:"_source"`
			Sort []int64 `json:"sort"`
		} `json:"hits"`
	} `json:"hits"`
}

// END of the Struct

// GetArticleById
type GetArticleById struct {
	Took     int  `json:"took"`
	TimedOut bool `json:"timed_out"`
	Shards   struct {
		Total      int `json:"total"`
		Successful int `json:"successful"`
		Skipped    int `json:"skipped"`
		Failed     int `json:"failed"`
	} `json:"_shards"`
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		MaxScore float64 `json:"max_score"`
		Hits     []struct {
			Index  string  `json:"_index"`
			ID     string  `json:"_id"`
			Score  float64 `json:"_score"`
			Source struct {
				ID               string    `json:"id"`
				Title            string    `json:"title"`
				ContentFormatted string    `json:"content_formatted"`
				ContentRaw       string    `json:"content_raw"`
				AuthorName       string    `json:"author_name"`
				AuthorID         string    `json:"author_id"`
				Published        bool      `json:"published"`
				Tags             []string  `json:"tags"`
				CreateTime       time.Time `json:"create_time"`
				UpdateTime       time.Time `json:"update_time"`
				CanEdit          bool      `json:"can_edit"`
				ContentOwnership int       `json:"content_ownership"`
				FolderPath       string    `json:"folder_path"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

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
