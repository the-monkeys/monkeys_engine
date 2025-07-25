package blog

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

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

func (asc *BlogServiceClient) SearchBlogs(ctx *gin.Context) {

	// Get limit and offset with defaults
	limit := ctx.DefaultQuery("limit", "20")
	offset := ctx.DefaultQuery("offset", "0")

	// Convert to integers
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 20
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	// Bind optional tags from request body
	var queryString Query
	if err := ctx.ShouldBindBodyWithJSON(&queryString); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "cannot bind the search query"})
		return
	}

	// Create search request
	searchReq := &pb.SearchReq{
		Query:  queryString.SearchQuery,
		Limit:  int32(limitInt),
		Offset: int32(offsetInt),
	}

	logrus.Debugf("Searching blogs with query: %s, limit: %d, offset: %d, query: %v", queryString.SearchQuery, limitInt, offsetInt, queryString.SearchQuery)

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
		"query":       queryString.SearchQuery,
		"limit":       limitInt,
		"offset":      offsetInt,
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
	stream, err := asc.Client.GetUsersBlogs(context.Background(), &pb.BlogListReq{
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
	stream, err := asc.Client.GetUsersBlogs(context.Background(), &pb.BlogListReq{
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
