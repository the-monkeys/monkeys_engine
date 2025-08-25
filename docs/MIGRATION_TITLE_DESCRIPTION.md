# Migration Guide for Title and Description Fields

## Overview
This document explains how to migrate existing blogs to populate the new dedicated title and description fields.

## Migration Strategy

### Option 1: Bulk Update via Elasticsearch (Recommended)
Use Elasticsearch's update by query API to populate title and description fields for existing blogs:

```bash
# Update all blogs without title field
curl -X POST "localhost:9200/blogs/_update_by_query" -H "Content-Type: application/json" -d '{
  "script": {
    "source": "
      if (ctx._source.blog.title == null || ctx._source.blog.title.isEmpty()) {
        for (block in ctx._source.blog.blocks) {
          if (block.type == \"header\" && block.data.level == 1) {
            ctx._source.blog.title = block.data.text;
            break;
          }
        }
      }
    "
  },
  "query": {
    "bool": {
      "must_not": [
        {
          "exists": {
            "field": "blog.title"
          }
        }
      ]
    }
  }
}'

# Update all blogs without description field
curl -X POST "localhost:9200/blogs/_update_by_query" -H "Content-Type: application/json" -d '{
  "script": {
    "source": "
      if (ctx._source.blog.description == null || ctx._source.blog.description.isEmpty()) {
        for (block in ctx._source.blog.blocks) {
          if (block.type == \"paragraph\") {
            ctx._source.blog.description = block.data.text;
            break;
          }
        }
      }
    "
  },
  "query": {
    "bool": {
      "must_not": [
        {
          "exists": {
            "field": "blog.description"
          }
        }
      ]
    }
  }
}'
```

### Option 2: Application-Level Migration
Create a migration service that:

1. Fetches all blogs without title/description fields
2. Extracts title from first header block (level 1)
3. Extracts description from first paragraph block
4. Updates the blog with these values

```go
func MigrateExistingBlogs(ctx context.Context, client ElasticsearchStorage) error {
    // Get all blogs without title field
    query := map[string]interface{}{
        "query": map[string]interface{}{
            "bool": map[string]interface{}{
                "must_not": []map[string]interface{}{
                    {
                        "exists": map[string]interface{}{
                            "field": "blog.title",
                        },
                    },
                },
            },
        },
    }
    
    // Process in batches
    // Extract title/description from blocks
    // Update blog with new fields
    
    return nil
}
```

### Option 3: Lazy Migration
The system already supports fallback to content extraction, so no immediate migration is required. Blogs will automatically use dedicated fields when they're set, and fall back to content extraction otherwise.

## Deployment Notes

1. **Backward Compatibility**: The system maintains full backward compatibility
2. **No Breaking Changes**: Existing blogs continue to work without modification
3. **Gradual Migration**: Can migrate blogs over time as they're edited
4. **Zero Downtime**: Migration can run while the system is live

## Verification

After migration, verify that:
- Blogs have populated title and description fields
- Metadata extraction returns dedicated fields instead of extracted content
- API responses include the new fields
- Search functionality works with both dedicated and extracted content

## Rollback Plan

If needed, the title and description fields can be removed without affecting existing functionality, as the system will fall back to content extraction.