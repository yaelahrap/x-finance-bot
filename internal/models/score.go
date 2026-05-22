package models

// Score is the multi-dimensional relevance score assigned to an article or draft.
// Each dimension is an integer (typically 0-100). TotalScore is the aggregate
// used for ranking and auto-post thresholds.
type Score struct {
	// IndonesiaRelevance measures how relevant the item is to Indonesian audiences.
	IndonesiaRelevance int `json:"indonesia_relevance"`
	// GlobalImportance measures broad geopolitical or macroeconomic importance.
	GlobalImportance int `json:"global_importance"`
	// MarketImpact measures expected impact on financial markets.
	MarketImpact int `json:"market_impact"`
	// Urgency measures time-sensitivity of the news.
	Urgency int `json:"urgency"`
	// PublicInterest measures expected reader engagement.
	PublicInterest int `json:"public_interest"`
	// SourceConfidence measures the reliability of the underlying source.
	SourceConfidence int `json:"source_confidence"`
	// TotalScore is the aggregated final score used for ranking decisions.
	TotalScore int `json:"total_score"`
}
