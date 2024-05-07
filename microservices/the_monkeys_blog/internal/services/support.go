package services

// import (
// 	"regexp"
// 	"sort"
// 	"strings"

// 	"github.com/microcosm-cc/bluemonday"
// 	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/models"
// 	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/pb"
// 	"google.golang.org/protobuf/types/known/timestamppb"
// )

// func parseToStruct(result models.Last100Articles) []pb.GetBlogsResponse {
// 	var resp []pb.GetBlogsResponse

// 	for _, val := range result.Hits.Hits {

// 		// Add content here
// 		res := pb.GetBlogsResponse{
// 			Id:         val.Source.ID,
// 			Title:      val.Source.Title,
// 			AuthorName: val.Source.AuthorName,
// 			AuthorId:   val.Source.AuthorID,
// 			Content:    strings.Replace(strings.Replace(val.Source.ContentRaw, "\t", " ", -1), "\n", "", -1),
// 			CreateTime: timestamppb.New(val.Source.CreateTime),
// 		}
// 		resp = append(resp, res)
// 	}

// 	return resp
// }

// func partialOrAllUpdate(isPartial bool, existingArt *pb.GetBlogByIdResponse, reqArt *pb.EditBlogRequest) *pb.EditBlogRequest {
// 	procdArt := &pb.EditBlogRequest{Id: reqArt.Id}

// 	if isPartial {
// 		if reqArt.Title == "" {
// 			procdArt.Title = existingArt.Title
// 		} else {
// 			procdArt.Title = reqArt.Title
// 		}
// 		if reqArt.Content == "" {
// 			procdArt.Content = existingArt.Content
// 		} else {
// 			procdArt.Content = reqArt.Content
// 		}
// 		if len(reqArt.Tags) == 0 {
// 			procdArt.Tags = existingArt.Tags
// 		} else {
// 			procdArt.Tags = reqArt.Tags
// 		}
// 	} else {
// 		procdArt.Title = reqArt.Title
// 		procdArt.Content = reqArt.Content
// 		procdArt.Tags = reqArt.Tags
// 	}

// 	return procdArt
// }

// func formattedToRawContent(content string) string {
// 	p := bluemonday.StripTagsPolicy()
// 	raw := p.Sanitize(content)

// 	return raw
// }

// func RemoveHtmlTag(in string) string {
// 	// regex to match html tag
// 	const pattern = `(<\/?[a-zA-A]+?[^>]*\/?>)*`
// 	r := regexp.MustCompile(pattern)
// 	groups := r.FindAllString(in, -1)
// 	// should replace long string first
// 	sort.Slice(groups, func(i, j int) bool {
// 		return len(groups[i]) > len(groups[j])
// 	})
// 	for _, group := range groups {
// 		if strings.TrimSpace(group) != "" {
// 			in = strings.ReplaceAll(in, group, "")
// 		}
// 	}
// 	return in
// }
