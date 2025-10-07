package database

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_activity/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Elasticsearch indices for different types of events
	ActivityEventIndex      = "activity-events"
	SecurityEventIndex      = "security-events"
	UserBehaviorIndex       = "user-behavior"
	ContentInteractionIndex = "content-interactions"
	RecommendationIndex     = "recommendation-interactions"
	NotificationIndex       = "notification-events"
	FinancialIndex          = "financial-events"
	IntegrationIndex        = "integration-events"
	IncidentIndex           = "incident-events"
	ComplianceIndex         = "compliance-events"
	SearchActivityIndex     = "search-activities"
	PerformanceIndex        = "performance-events"
	UserJourneyIndex        = "user-journey-events"
	ReadingBehaviorIndex    = "reading-behavior"
)

// ActivityDB handles Elasticsearch operations for activity tracking
type ActivityDB struct {
	client *elasticsearch.Client
	logger *zap.SugaredLogger
	config *config.Config
}

// NewActivityDB creates a new ActivityDB instance
func NewActivityDB(cfg *config.Config, logger *zap.SugaredLogger) (*ActivityDB, error) {
	// Elasticsearch configuration
	esConfig := elasticsearch.Config{
		Addresses: []string{
			cfg.Opensearch.Host, // Host already includes protocol
		},
		Username: cfg.Opensearch.Username,
		Password: cfg.Opensearch.Password,
	}

	client, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	db := &ActivityDB{
		client: client,
		logger: logger,
		config: cfg,
	}

	// Initialize indices
	if err := db.initializeIndices(); err != nil {
		return nil, fmt.Errorf("failed to initialize indices: %w", err)
	}

	return db, nil
}

// initializeIndices creates the necessary Elasticsearch indices with proper mappings
func (db *ActivityDB) initializeIndices() error {
	indices := []string{
		ActivityEventIndex,
		SecurityEventIndex,
		UserBehaviorIndex,
		ContentInteractionIndex,
		RecommendationIndex,
		NotificationIndex,
		FinancialIndex,
		IntegrationIndex,
		IncidentIndex,
		ComplianceIndex,
		SearchActivityIndex,
		PerformanceIndex,
		UserJourneyIndex,
		ReadingBehaviorIndex,
	}

	for _, index := range indices {
		if err := db.createIndexIfNotExists(index); err != nil {
			return fmt.Errorf("failed to create index %s: %w", index, err)
		}
	}

	return nil
}

// createIndexIfNotExists creates an index if it doesn't exist
func (db *ActivityDB) createIndexIfNotExists(indexName string) error {
	// Check if index exists
	req := esapi.IndicesExistsRequest{
		Index: []string{indexName},
	}

	res, err := req.Do(context.Background(), db.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// If index exists (200), return
	if res.StatusCode == 200 {
		db.logger.Debugw("Index already exists", "index", indexName)
		return nil
	}

	// Create index with time-series optimized mapping
	mapping := db.getIndexMapping(indexName)

	createReq := esapi.IndicesCreateRequest{
		Index: indexName,
		Body:  strings.NewReader(mapping),
	}

	createRes, err := createReq.Do(context.Background(), db.client)
	if err != nil {
		return err
	}
	defer createRes.Body.Close()

	if createRes.IsError() {
		return fmt.Errorf("failed to create index %s: %s", indexName, createRes.String())
	}

	db.logger.Infow("Created Elasticsearch index", "index", indexName)
	return nil
}

// getIndexMapping returns the mapping configuration for different indices
func (db *ActivityDB) getIndexMapping(indexName string) string {
	// Base mapping for time-series data
	baseMapping := `{
		"settings": {
			"number_of_shards": 1,
			"number_of_replicas": 1,
			"index": {
				"refresh_interval": "5s"
			}
		},
		"mappings": {
			"properties": {
				"@timestamp": {
					"type": "date",
					"format": "strict_date_optional_time||epoch_millis"
				},
				"user_id": {
					"type": "keyword"
				},
				"account_id": {
					"type": "keyword"
				},
				"session_id": {
					"type": "keyword"
				},
				"client_ip": {
					"type": "ip"
				},
				"country": {
					"type": "keyword"
				},
				"user_agent": {
					"type": "text",
					"fields": {
						"keyword": {
							"type": "keyword",
							"ignore_above": 256
						}
					}
				}
			}
		}
	}`

	// You can customize mappings per index type if needed
	switch indexName {
	case ActivityEventIndex:
		return fmt.Sprintf(`{
			"settings": {
				"number_of_shards": 2,
				"number_of_replicas": 1,
				"index": {
					"refresh_interval": "1s"
				}
			},
			"mappings": {
				"properties": {
					"@timestamp": {"type": "date"},
					"user_id": {"type": "keyword"},
					"account_id": {"type": "keyword"},
					"session_id": {"type": "keyword"},
					"category": {"type": "keyword"},
					"action": {"type": "keyword"},
					"resource": {"type": "keyword"},
					"resource_id": {"type": "keyword"},
					"client_ip": {"type": "ip"},
					"user_agent": {"type": "text"},
					"country": {"type": "keyword"},
					"platform": {"type": "keyword"},
					"referrer": {"type": "text"},
					"success": {"type": "boolean"},
					"duration_ms": {"type": "long"},
					"metadata": {"type": "object", "enabled": false}
				}
			}
		}`)
	case SecurityEventIndex:
		return fmt.Sprintf(`{
			"settings": {
				"number_of_shards": 1,
				"number_of_replicas": 1
			},
			"mappings": {
				"properties": {
					"@timestamp": {"type": "date"},
					"user_id": {"type": "keyword"},
					"account_id": {"type": "keyword"},
					"event_type": {"type": "keyword"},
					"severity": {"type": "keyword"},
					"description": {"type": "text"},
					"risk_score": {"type": "integer"},
					"resolved": {"type": "boolean"},
					"resolved_by": {"type": "keyword"},
					"resolved_at": {"type": "date"},
					"context": {"type": "object"}
				}
			}
		}`)
	default:
		return baseMapping
	}
}

// SaveActivity saves an activity event to Elasticsearch
func (db *ActivityDB) SaveActivity(ctx context.Context, req *pb.TrackActivityRequest) (string, error) {
	activityID := fmt.Sprintf("activity_%d_%s", time.Now().UnixNano(), req.GetUserId())

	// Convert protobuf to document
	doc := map[string]interface{}{
		"@timestamp":  time.Now().Format(time.RFC3339),
		"id":          activityID,
		"user_id":     req.GetUserId(),
		"account_id":  req.GetAccountId(),
		"session_id":  req.GetSessionId(),
		"category":    req.GetCategory().String(),
		"action":      req.GetAction(),
		"resource":    req.GetResource(),
		"resource_id": req.GetResourceId(),
		"client_ip":   req.GetClientIp(),
		"user_agent":  req.GetUserAgent(),
		"country":     req.GetCountry(),
		"platform":    req.GetPlatform().String(),
		"referrer":    req.GetReferrer(),
		"success":     req.GetSuccess(),
		"duration_ms": req.GetDurationMs(),
		"metadata":    req.GetMetadata(),
	}

	docBytes, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("failed to marshal document: %w", err)
	}

	// Index the document
	indexReq := esapi.IndexRequest{
		Index:      ActivityEventIndex,
		DocumentID: activityID,
		Body:       bytes.NewReader(docBytes),
		Refresh:    "true",
	}

	res, err := indexReq.Do(ctx, db.client)
	if err != nil {
		return "", fmt.Errorf("failed to index document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return "", fmt.Errorf("indexing failed: %s", res.String())
	}

	db.logger.Debugw("Activity saved to Elasticsearch",
		"activity_id", activityID,
		"user_id", req.GetUserId(),
		"category", req.GetCategory(),
		"action", req.GetAction(),
	)

	return activityID, nil
}

// GetUserActivities retrieves user activities from Elasticsearch
func (db *ActivityDB) GetUserActivities(ctx context.Context, req *pb.GetUserActivitiesRequest) ([]*pb.ActivityEvent, int64, error) {
	// Build Elasticsearch query
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"user_id": req.GetUserId(),
						},
					},
				},
			},
		},
		"sort": []map[string]interface{}{
			{
				"@timestamp": map[string]interface{}{
					"order": "desc",
				},
			},
		},
		"size": req.GetLimit(),
		"from": req.GetOffset(),
	}

	// Add optional filters
	if req.GetAccountId() != "" {
		mustFilters := query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"].([]map[string]interface{})
		mustFilters = append(mustFilters, map[string]interface{}{
			"term": map[string]interface{}{
				"account_id": req.GetAccountId(),
			},
		})
		query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"] = mustFilters
	}

	if req.GetCategory() != pb.ActivityCategory_CATEGORY_UNSPECIFIED {
		mustFilters := query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"].([]map[string]interface{})
		mustFilters = append(mustFilters, map[string]interface{}{
			"term": map[string]interface{}{
				"category": req.GetCategory().String(),
			},
		})
		query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"] = mustFilters
	}

	// Add time range filter if provided
	if req.GetStartTime() != nil || req.GetEndTime() != nil {
		rangeFilter := map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": map[string]interface{}{},
			},
		}

		if req.GetStartTime() != nil {
			rangeFilter["range"].(map[string]interface{})["@timestamp"].(map[string]interface{})["gte"] = req.GetStartTime().AsTime().Format(time.RFC3339)
		}

		if req.GetEndTime() != nil {
			rangeFilter["range"].(map[string]interface{})["@timestamp"].(map[string]interface{})["lte"] = req.GetEndTime().AsTime().Format(time.RFC3339)
		}

		mustFilters := query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"].([]map[string]interface{})
		mustFilters = append(mustFilters, rangeFilter)
		query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"] = mustFilters
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal query: %w", err)
	}

	// Execute search
	searchReq := esapi.SearchRequest{
		Index: []string{ActivityEventIndex},
		Body:  bytes.NewReader(queryBytes),
	}

	res, err := searchReq.Do(ctx, db.client)
	if err != nil {
		return nil, 0, fmt.Errorf("search failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, 0, fmt.Errorf("search error: %s", res.String())
	}

	// Parse response
	var searchResponse struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source map[string]interface{} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&searchResponse); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to protobuf
	activities := make([]*pb.ActivityEvent, 0, len(searchResponse.Hits.Hits))
	for _, hit := range searchResponse.Hits.Hits {
		activity := db.convertToActivityEvent(hit.Source)
		activities = append(activities, activity)
	}

	return activities, searchResponse.Hits.Total.Value, nil
}

// convertToActivityEvent converts Elasticsearch document to protobuf ActivityEvent
func (db *ActivityDB) convertToActivityEvent(doc map[string]interface{}) *pb.ActivityEvent {
	activity := &pb.ActivityEvent{}

	if id, ok := doc["id"].(string); ok {
		activity.Id = id
	}
	if userID, ok := doc["user_id"].(string); ok {
		activity.UserId = userID
	}
	if accountID, ok := doc["account_id"].(string); ok {
		activity.AccountId = accountID
	}
	if sessionID, ok := doc["session_id"].(string); ok {
		activity.SessionId = sessionID
	}
	if category, ok := doc["category"].(string); ok {
		// Convert string back to enum
		activity.Category = db.stringToActivityCategory(category)
	}
	if action, ok := doc["action"].(string); ok {
		activity.Action = action
	}
	if resource, ok := doc["resource"].(string); ok {
		activity.Resource = resource
	}
	if resourceID, ok := doc["resource_id"].(string); ok {
		activity.ResourceId = resourceID
	}
	if clientIP, ok := doc["client_ip"].(string); ok {
		activity.ClientIp = clientIP
	}
	if userAgent, ok := doc["user_agent"].(string); ok {
		activity.UserAgent = userAgent
	}
	if country, ok := doc["country"].(string); ok {
		activity.Country = country
	}
	if platform, ok := doc["platform"].(string); ok {
		activity.Platform = db.stringToPlatform(platform)
	}
	if referrer, ok := doc["referrer"].(string); ok {
		activity.Referrer = referrer
	}
	if success, ok := doc["success"].(bool); ok {
		activity.Success = success
	}
	if duration, ok := doc["duration_ms"].(float64); ok {
		activity.DurationMs = int64(duration)
	}
	if timestamp, ok := doc["@timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			activity.Timestamp = timestamppb.New(t)
			activity.CreatedAt = timestamppb.New(t)
		}
	}

	return activity
}

// SaveSecurityEvent saves a security event to Elasticsearch
func (db *ActivityDB) SaveSecurityEvent(ctx context.Context, req *pb.TrackSecurityEventRequest) (string, error) {
	eventID := fmt.Sprintf("security_%d_%s", time.Now().UnixNano(), req.GetUserId())

	doc := map[string]interface{}{
		"@timestamp":  time.Now().Format(time.RFC3339),
		"id":          eventID,
		"user_id":     req.GetUserId(),
		"account_id":  req.GetAccountId(),
		"event_type":  req.GetEventType(),
		"severity":    req.GetSeverity().String(),
		"description": req.GetDescription(),
		"risk_score":  req.GetRiskScore(),
		"resolved":    false,
		"context":     req.GetContext(),
	}

	docBytes, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("failed to marshal security event: %w", err)
	}

	indexReq := esapi.IndexRequest{
		Index:      SecurityEventIndex,
		DocumentID: eventID,
		Body:       bytes.NewReader(docBytes),
		Refresh:    "true",
	}

	res, err := indexReq.Do(ctx, db.client)
	if err != nil {
		return "", fmt.Errorf("failed to index security event: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return "", fmt.Errorf("security event indexing failed: %s", res.String())
	}

	db.logger.Warnw("Security event saved to Elasticsearch",
		"event_id", eventID,
		"user_id", req.GetUserId(),
		"event_type", req.GetEventType(),
		"severity", req.GetSeverity(),
	)

	return eventID, nil
}

// Health checks Elasticsearch connectivity
func (db *ActivityDB) Health(ctx context.Context) error {
	res, err := db.client.Info()
	if err != nil {
		return fmt.Errorf("elasticsearch health check failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("elasticsearch is not healthy: %s", res.String())
	}

	return nil
}

// Helper method to convert category string back to enum
func (db *ActivityDB) stringToActivityCategory(categoryStr string) pb.ActivityCategory {
	switch categoryStr {
	case "CATEGORY_AUTHENTICATION":
		return pb.ActivityCategory_CATEGORY_AUTHENTICATION
	case "CATEGORY_AUTHORIZATION":
		return pb.ActivityCategory_CATEGORY_AUTHORIZATION
	case "CATEGORY_CONTENT":
		return pb.ActivityCategory_CATEGORY_CONTENT
	case "CATEGORY_SOCIAL":
		return pb.ActivityCategory_CATEGORY_SOCIAL
	case "CATEGORY_SEARCH":
		return pb.ActivityCategory_CATEGORY_SEARCH
	case "CATEGORY_NAVIGATION":
		return pb.ActivityCategory_CATEGORY_NAVIGATION
	case "CATEGORY_RECOMMENDATION":
		return pb.ActivityCategory_CATEGORY_RECOMMENDATION
	case "CATEGORY_ANALYTICS":
		return pb.ActivityCategory_CATEGORY_ANALYTICS
	case "CATEGORY_SECURITY":
		return pb.ActivityCategory_CATEGORY_SECURITY
	case "CATEGORY_SYSTEM":
		return pb.ActivityCategory_CATEGORY_SYSTEM
	case "CATEGORY_NOTIFICATION":
		return pb.ActivityCategory_CATEGORY_NOTIFICATION
	case "CATEGORY_COMPLIANCE":
		return pb.ActivityCategory_CATEGORY_COMPLIANCE
	case "CATEGORY_FINANCIAL":
		return pb.ActivityCategory_CATEGORY_FINANCIAL
	case "CATEGORY_INTEGRATION":
		return pb.ActivityCategory_CATEGORY_INTEGRATION
	case "CATEGORY_INCIDENT":
		return pb.ActivityCategory_CATEGORY_INCIDENT
	default:
		return pb.ActivityCategory_CATEGORY_UNSPECIFIED
	}
}

// Helper method to convert platform string back to enum
func (db *ActivityDB) stringToPlatform(platformStr string) pb.Platform {
	switch platformStr {
	case "PLATFORM_WEB":
		return pb.Platform_PLATFORM_WEB
	case "PLATFORM_MOBILE":
		return pb.Platform_PLATFORM_MOBILE
	case "PLATFORM_TABLET":
		return pb.Platform_PLATFORM_TABLET
	case "PLATFORM_API":
		return pb.Platform_PLATFORM_API
	case "PLATFORM_DESKTOP":
		return pb.Platform_PLATFORM_DESKTOP
	default:
		return pb.Platform_PLATFORM_UNSPECIFIED
	}
}
