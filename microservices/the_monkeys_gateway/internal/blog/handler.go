package blog

import (
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/constants"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (asc *BlogServiceClient) GetFeedPostsMeta(ctx *gin.Context) {
	// Get Limits and Offset
	limit := ctx.DefaultQuery("limit", "500")
	offset := ctx.DefaultQuery("offset", "0")

	// Convert to int
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 500
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	// Bind tags from request body
	var tags Tags
	if err := ctx.ShouldBindBodyWithJSON(&tags); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "cannot bind the tags"})
		return
	}
	logrus.Infof("✅ Fetching feed from blog service")

	// Call gRPC to get blog metadata
	stream, err := asc.Client.GetBlogsMetadata(context.Background(), &pb.BlogListReq{
		Limit:  int32(limitInt),
		Offset: int32(offsetInt),
		Tags:   tags.Tags,
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the blogs for the given tags"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs by tags"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	var totalBlogs int // Store total number of blogs

	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the given tags"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		// Unmarshal into a map since response structure has changed
		var blogMap map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMap); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}

		// Extract "total_blogs" if present
		if total, ok := blogMap["total_blogs"].(float64); ok { // JSON numbers default to float64
			totalBlogs = int(total)
		}

		// Extract the "blogs" array safely
		blogsData, ok := blogMap["blogs"]
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "missing blogs data in response"})
			return
		}

		// Convert blogsData to []map[string]interface{}
		blogList, ok := blogsData.([]interface{})
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "invalid blogs format in response"})
			return
		}

		// Convert []interface{} to []map[string]interface{}
		for _, b := range blogList {
			if blogEntry, valid := b.(map[string]interface{}); valid {
				allBlogs = append(allBlogs, blogEntry)
			}
		}
	}

	// Add additional metadata (like & bookmark count) for each blog
	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.UserCli.GetNoOfLikeCounts(blogID)
		blog["like_count"] = likeCount

		bookmarkCount, _ := asc.UserCli.GetNoOfBookmarkCounts(blogID)
		blog["bookmark_count"] = bookmarkCount
	}

	// Final response including total blogs count
	responseBlogs := map[string]interface{}{
		"total_blogs": totalBlogs,
		"blogs":       allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) GetsMetaFeed(ctx *gin.Context) {
	// Get Limits and Offset
	limit := ctx.DefaultQuery("limit", "500")
	offset := ctx.DefaultQuery("offset", "0")

	// Convert to int
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 500
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	// Call gRPC to get blog metadata
	stream, err := asc.Client.GetBlogsMetadata(context.Background(), &pb.BlogListReq{
		Limit:  int32(limitInt),
		Offset: int32(offsetInt),
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the blogs for the given tags"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs by tags"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	var totalBlogs int // Store total number of blogs

	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the given tags"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		// Unmarshal into a map since response structure has changed
		var blogMap map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMap); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}

		// Extract "total_blogs" if present
		if total, ok := blogMap["total_blogs"].(float64); ok { // JSON numbers default to float64
			totalBlogs = int(total)
		}

		// Extract the "blogs" array safely
		blogsData, ok := blogMap["blogs"]
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "missing blogs data in response"})
			return
		}

		// Convert blogsData to []map[string]interface{}
		blogList, ok := blogsData.([]interface{})
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "invalid blogs format in response"})
			return
		}

		// Convert []interface{} to []map[string]interface{}
		for _, b := range blogList {
			if blogEntry, valid := b.(map[string]interface{}); valid {
				allBlogs = append(allBlogs, blogEntry)
			}
		}
	}

	// Add additional metadata (like & bookmark count) for each blog
	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.UserCli.GetNoOfLikeCounts(blogID)
		blog["like_count"] = likeCount

		bookmarkCount, _ := asc.UserCli.GetNoOfBookmarkCounts(blogID)
		blog["bookmark_count"] = bookmarkCount
	}

	// Final response including total blogs count
	responseBlogs := map[string]interface{}{
		"total_blogs": totalBlogs,
		"blogs":       allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) SearchBlogsQuery(ctx *gin.Context) {
	// Get limit and offset with defaults
	limit := ctx.DefaultQuery("limit", "20")
	offset := ctx.DefaultQuery("offset", "0")

	// Get search query from URL parameter
	searchQuery := ctx.Query("search_term")
	if searchQuery == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "search_term query parameter is required"})
		return
	}

	// Convert to integers
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 20
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	// Create search request
	searchReq := &pb.SearchReq{
		Query:  searchQuery,
		Limit:  int32(limitInt),
		Offset: int32(offsetInt),
	}

	logrus.Debugf("Searching blogs with query: %s, limit: %d, offset: %d", searchQuery, limitInt, offsetInt)

	// Call gRPC to search blog metadata
	stream, err := asc.Client.SearchBlogsMetadata(context.Background(), searchReq)
	if err != nil {
		logrus.Errorf("cannot search blogs, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the search query"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "server error during search"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	var totalBlogs int

	// Process search results stream
	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.Errorf("error receiving search results: %v", err)
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the search query"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		// Unmarshal into a map
		var blogMap map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMap); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}

		// Extract "total_blogs" if present
		if total, ok := blogMap["total_blogs"].(float64); ok {
			totalBlogs = int(total)
		}

		// Extract the "blogs" array safely
		blogsData, ok := blogMap["blogs"]
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "missing blogs data in response"})
			return
		}

		// Convert blogsData to []map[string]interface{}
		blogList, ok := blogsData.([]interface{})
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "invalid blogs format in response"})
			return
		}

		// Convert []interface{} to []map[string]interface{}
		for _, b := range blogList {
			if blogEntry, valid := b.(map[string]interface{}); valid {
				allBlogs = append(allBlogs, blogEntry)
			}
		}
	}

	// Add additional metadata (like & bookmark count) for each blog
	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.UserCli.GetNoOfLikeCounts(blogID)
		blog["like_count"] = likeCount

		bookmarkCount, _ := asc.UserCli.GetNoOfBookmarkCounts(blogID)
		blog["bookmark_count"] = bookmarkCount
	}

	// Final response including total blogs count
	responseBlogs := map[string]interface{}{
		"total_blogs": totalBlogs,
		"blogs":       allBlogs,
		"query":       searchQuery,
		"limit":       limitInt,
		"offset":      offsetInt,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) GetBusinessNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.Business)
}

func (asc *BlogServiceClient) GetTechnologyNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.Technology)
}

func (asc *BlogServiceClient) GetScienceNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.Science)
}

func (asc *BlogServiceClient) GetHealthNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.Health)
}

func (asc *BlogServiceClient) GetSportsNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.Sports)
}

func (asc *BlogServiceClient) GetEntertainmentNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.Entertainment)
}

func (asc *BlogServiceClient) GetTravelNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.Travel)
}

func (asc *BlogServiceClient) GetFoodNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.Food)
}

func (asc *BlogServiceClient) GetLifestyleNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.Lifestyle)
}

func (asc *BlogServiceClient) GetEducationNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.Education)
}

func (asc *BlogServiceClient) GetSpaceNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.Space)
}

func (asc *BlogServiceClient) GetPsychologyNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.PhilosophyAndPsychology)
}

func (asc *BlogServiceClient) GetHumorNews(ctx *gin.Context) {
	asc.getNewsByCategory(ctx, constants.Humor)
}

func (asc *BlogServiceClient) getNewsByCategory(ctx *gin.Context, tags []string) {
	// Get Limits and offset
	limit := ctx.DefaultQuery("limit", "500")
	offset := ctx.DefaultQuery("offset", "0")
	// Convert to int
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 100
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	// Call gRPC to get blog metadata
	stream, err := asc.Client.GetBlogsMetadata(context.Background(), &pb.BlogListReq{
		Limit:  int32(limitInt),
		Offset: int32(offsetInt),
		Tags:   tags,
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the blogs for the given tags"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs by tags"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	var totalBlogs int // Store total number of blogs

	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the given tags"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		// Unmarshal into a map since response structure has changed
		var blogMap map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMap); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}

		// Extract "total_blogs" if present
		if total, ok := blogMap["total_blogs"].(float64); ok { // JSON numbers default to float64
			totalBlogs = int(total)
		}

		// Extract the "blogs" array safely
		blogsData, ok := blogMap["blogs"]
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "missing blogs data in response"})
			return
		}

		// Convert blogsData to []map[string]interface{}
		blogList, ok := blogsData.([]interface{})
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "invalid blogs format in response"})
			return
		}

		// Convert []interface{} to []map[string]interface{}
		for _, b := range blogList {
			if blogEntry, valid := b.(map[string]interface{}); valid {
				allBlogs = append(allBlogs, blogEntry)
			}
		}
	}

	// Add additional metadata (like & bookmark count) for each blog
	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.UserCli.GetNoOfLikeCounts(blogID)
		blog["like_count"] = likeCount

		bookmarkCount, _ := asc.UserCli.GetNoOfBookmarkCounts(blogID)
		blog["bookmark_count"] = bookmarkCount
	}

	// Final response including total blogs count
	responseBlogs := map[string]interface{}{
		"total_blogs": totalBlogs,
		"blogs":       allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) MetaUsersPublished(ctx *gin.Context) {
	userName := ctx.Param("username")

	// Get the account_id from the username
	userInfo, err := asc.UserCli.GetUserDetails(userName)
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the user does not exist"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot fetch the user details"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	// Get Limits and Offset
	limit := ctx.DefaultQuery("limit", "500")
	offset := ctx.DefaultQuery("offset", "0")

	// Convert to int
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 500
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	// Call gRPC to get blog metadata
	stream, err := asc.Client.MetaGetUsersBlogs(context.Background(), &pb.BlogListReq{
		AccountId: userInfo.AccountId,
		IsDraft:   false, // Only published blogs
		Limit:     int32(limitInt),
		Offset:    int32(offsetInt),
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the blogs for the given tags"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs by tags"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	var totalBlogs int // Store total number of blogs

	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the given tags"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		// Unmarshal into a map since response structure has changed
		var blogMap map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMap); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}

		// Extract "total_blogs" if present
		if total, ok := blogMap["total_blogs"].(float64); ok { // JSON numbers default to float64
			totalBlogs = int(total)
		}

		// Extract the "blogs" array safely
		blogsData, ok := blogMap["blogs"]
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "missing blogs data in response"})
			return
		}

		// Convert blogsData to []map[string]interface{}
		blogList, ok := blogsData.([]interface{})
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "invalid blogs format in response"})
			return
		}

		// Convert []interface{} to []map[string]interface{}
		for _, b := range blogList {
			if blogEntry, valid := b.(map[string]interface{}); valid {
				allBlogs = append(allBlogs, blogEntry)
			}
		}
	}

	// Add additional metadata (like & bookmark count) for each blog
	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.UserCli.GetNoOfLikeCounts(blogID)
		blog["like_count"] = likeCount

		bookmarkCount, _ := asc.UserCli.GetNoOfBookmarkCounts(blogID)
		blog["bookmark_count"] = bookmarkCount
	}

	// Final response including total blogs count
	responseBlogs := map[string]interface{}{
		"total_blogs": totalBlogs,
		"blogs":       allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) MetaMyDraftBlogs(ctx *gin.Context) {
	username := ctx.GetString("userName")

	// Get the account_id from the username
	userInfo, err := asc.UserCli.GetUserDetails(username)
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the user does not exist"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot fetch the user details"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	// Get Limits and Offset
	limit := ctx.DefaultQuery("limit", "500")
	offset := ctx.DefaultQuery("offset", "0")

	// Convert to int
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 500
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	// Call gRPC to get blog metadata
	stream, err := asc.Client.MetaGetUsersBlogs(context.Background(), &pb.BlogListReq{
		AccountId: userInfo.AccountId,
		IsDraft:   true, // Only draft blogs
		Limit:     int32(limitInt),
		Offset:    int32(offsetInt),
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the blogs for the given tags"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs by tags"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	var totalBlogs int // Store total number of blogs

	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the given tags"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		// Unmarshal into a map since response structure has changed
		var blogMap map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMap); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}

		// Extract "total_blogs" if present
		if total, ok := blogMap["total_blogs"].(float64); ok { // JSON numbers default to float64
			totalBlogs = int(total)
		}

		// Extract the "blogs" array safely
		blogsData, ok := blogMap["blogs"]
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "missing blogs data in response"})
			return
		}

		// Convert blogsData to []map[string]interface{}
		blogList, ok := blogsData.([]interface{})
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "invalid blogs format in response"})
			return
		}

		// Convert []interface{} to []map[string]interface{}
		for _, b := range blogList {
			if blogEntry, valid := b.(map[string]interface{}); valid {
				allBlogs = append(allBlogs, blogEntry)
			}
		}
	}

	// Add additional metadata (like & bookmark count) for each blog
	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.UserCli.GetNoOfLikeCounts(blogID)
		blog["like_count"] = likeCount

		bookmarkCount, _ := asc.UserCli.GetNoOfBookmarkCounts(blogID)
		blog["bookmark_count"] = bookmarkCount
	}

	// Final response including total blogs count
	responseBlogs := map[string]interface{}{
		"total_blogs": totalBlogs,
		"blogs":       allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) MetaMyBookmarks(ctx *gin.Context) {
	tokenAccountId := ctx.GetString("userName")

	// Get limit and offset and convert into int32
	limit := ctx.DefaultQuery("limit", "10")
	offset := ctx.DefaultQuery("offset", "0")
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 10
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	// Get all the draft blogs by my username
	blogResp, err := asc.UserCli.GetUsersBookmarks(tokenAccountId)

	if err != nil {
		logrus.Errorf("cannot get the bookmarks, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the bookmarks"})
		return
	}

	if len(blogResp) == 0 {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found"})
		return
	}

	// Call gRPC to get blog metadata
	stream, err := asc.Client.MetaGetBlogsByBlogIds(context.Background(), &pb.BlogListReq{
		BlogIds: blogResp,
		IsDraft: false, // Only published blogs
		Limit:   int32(limitInt),
		Offset:  int32(offsetInt),
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the blogs for the given tags"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs by tags"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	var totalBlogs int // Store total number of blogs

	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the given tags"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		// Unmarshal into a map since response structure has changed
		var blogMap map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMap); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}

		// Extract "total_blogs" if present
		if total, ok := blogMap["total_blogs"].(float64); ok { // JSON numbers default to float64
			totalBlogs = int(total)
		}

		// Extract the "blogs" array safely
		blogsData, ok := blogMap["blogs"]
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "missing blogs data in response"})
			return
		}

		// Convert blogsData to []map[string]interface{}
		blogList, ok := blogsData.([]interface{})
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "invalid blogs format in response"})
			return
		}

		// Convert []interface{} to []map[string]interface{}
		for _, b := range blogList {
			if blogEntry, valid := b.(map[string]interface{}); valid {
				allBlogs = append(allBlogs, blogEntry)
			}
		}
	}

	// Add additional metadata (like & bookmark count) for each blog
	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.UserCli.GetNoOfLikeCounts(blogID)
		blog["like_count"] = likeCount

		bookmarkCount, _ := asc.UserCli.GetNoOfBookmarkCounts(blogID)
		blog["bookmark_count"] = bookmarkCount
	}

	// Final response including total blogs count
	responseBlogs := map[string]interface{}{
		"total_blogs": totalBlogs,
		"blogs":       allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) GetUserTags(ctx *gin.Context) {
	username := ctx.Param("username")

	// Get the account_id from the username to verify user exists
	userInfo, err := asc.UserCli.GetUserDetails(username)
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the user does not exist"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot fetch the user details"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	resp, err := asc.Client.UsersBlogData(context.Background(), &pb.BlogReq{
		AccountId: userInfo.AccountId,
	})
	if err != nil {
		logrus.Errorf("cannot fetch user blog data, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot fetch user blog data"})
		return
	}

	var respMap map[string]interface{}
	if err := json.Unmarshal(resp.Value, &respMap); err != nil {
		logrus.Errorf("cannot unmarshal the blog response, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog response"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"tags": respMap})
}

// generateRandomWordCloud creates a random word cloud with up to maxWords
func generateRandomWordCloud(maxWords int) map[string]int {
	// Predefined list of words for the word cloud
	allWords := []string{
		"technology", "health", "business", "science", "lifestyle", "travel",
		"entertainment", "food", "education", "space", "psychology", "philosophy",
		"innovation", "research", "development", "artificial", "intelligence",
		"machine", "learning", "data", "analytics", "programming", "software",
		"hardware", "networking", "security", "blockchain", "cryptocurrency",
		"finance", "investment", "economics", "marketing", "design", "creativity",
		"art", "music", "literature", "history", "culture", "society",
		"environment", "sustainability", "climate", "energy", "renewable",
		"medicine", "fitness", "nutrition", "wellness", "mindfulness",
		"meditation", "productivity", "leadership", "management", "teamwork",
		"communication", "writing", "reading", "knowledge", "wisdom",
		"discovery", "exploration", "adventure", "nature", "wildlife",
		"photography", "video", "gaming", "sports", "competition",
		"achievement", "success", "motivation", "inspiration", "goals",
		"dreams", "future", "progress", "evolution", "transformation",
	}

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	// Determine actual number of words to include (random between 15-40)
	numWords := rand.Intn(26) + 15 // Random between 15 and 40

	if numWords > maxWords {
		numWords = maxWords
	}

	if numWords > len(allWords) {
		numWords = len(allWords)
	}

	// Shuffle the words array
	shuffledWords := make([]string, len(allWords))
	copy(shuffledWords, allWords)
	rand.Shuffle(len(shuffledWords), func(i, j int) {
		shuffledWords[i], shuffledWords[j] = shuffledWords[j], shuffledWords[i]
	})

	// Create word cloud with random frequencies
	wordCloud := make(map[string]int)
	for i := 0; i < numWords; i++ {
		// Random frequency between 1 and 20
		frequency := rand.Intn(20) + 1
		wordCloud[shuffledWords[i]] = frequency
	}

	return wordCloud
}
