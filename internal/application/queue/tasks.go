package queue

const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	QueueLow      = "low"

	TypeMetricCalculateNeighborhood = "metric.calculate_neighborhood"

	TypeCollectionFetchSource            = "collection.fetch_source"
	TypeCollectionNormalizeListing       = "collection.normalize_listing"
	TypeCollectionDeduplicateListing     = "collection.deduplicate_listing"
	TypeDecisionRefreshWindow            = "decision.refresh_window"
	TypeNotificationGenerateWeeklyReport = "notification.generate_weekly_report"
	TypeNotificationSendAlert            = "notification.send_alert"
)

type MetricCalculateNeighborhoodPayload struct {
	NeighborhoodID  string `json:"neighborhoodId"`
	CollectionRunID string `json:"collectionRunId,omitempty"`
	SourceID        string `json:"sourceId"`
}
