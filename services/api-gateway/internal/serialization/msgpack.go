package serialization

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vmihailenco/msgpack/v5"
)

// MessagePackRenderer provides MessagePack serialization for Gin
type MessagePackRenderer struct{}

func (r MessagePackRenderer) Render(w http.ResponseWriter, code int, data interface{}) error {
	w.Header().Set("Content-Type", "application/msgpack")
	w.WriteHeader(code)
	return msgpack.NewEncoder(w).Encode(data)
}

func (r MessagePackRenderer) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/msgpack")
}

// RespondWithMessagePack sends a MessagePack response (2x faster than JSON)
func RespondWithMessagePack(c *gin.Context, code int, obj interface{}) {
	c.Render(code, MessagePackRenderer{}, obj)
}

// RespondWithJSON provides JSON fallback for compatibility
func RespondWithJSON(c *gin.Context, code int, obj interface{}) {
	c.JSON(code, obj)
}

// RespondAuto automatically chooses the best format based on Accept header
func RespondAuto(c *gin.Context, code int, obj interface{}) {
	accept := c.GetHeader("Accept")

	// Prefer MessagePack for better performance
	switch {
	case accept == "application/msgpack":
		RespondWithMessagePack(c, code, obj)
	case accept == "application/json":
		RespondWithJSON(c, code, obj)
	default:
		// Default to MessagePack for internal services, JSON for external
		userAgent := c.GetHeader("User-Agent")
		if userAgent == "TradeCaptain-Internal" {
			RespondWithMessagePack(c, code, obj)
		} else {
			RespondWithJSON(c, code, obj)
		}
	}
}

// BindMessagePack binds MessagePack request body to struct
func BindMessagePack(c *gin.Context, obj interface{}) error {
	contentType := c.GetHeader("Content-Type")

	if contentType == "application/msgpack" {
		body, err := c.GetRawData()
		if err != nil {
			return err
		}
		return msgpack.Unmarshal(body, obj)
	}

	// Fallback to JSON
	return c.ShouldBindJSON(obj)
}

// Performance comparison middleware
func SerializationBenchmarkMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Add performance hints in response headers
		c.Header("X-Serialization-Hint", "Use 'Accept: application/msgpack' for 2x faster responses")
	}
}

// MarketData represents a market data point optimized for serialization
type MarketData struct {
	Symbol    string  `json:"symbol" msgpack:"symbol"`
	Price     float64 `json:"price" msgpack:"price"`
	Volume    uint64  `json:"volume" msgpack:"volume"`
	Timestamp int64   `json:"timestamp" msgpack:"timestamp"`
	Bid       float64 `json:"bid" msgpack:"bid"`
	Ask       float64 `json:"ask" msgpack:"ask"`
	High      float64 `json:"high" msgpack:"high"`
	Low       float64 `json:"low" msgpack:"low"`
	Open      float64 `json:"open" msgpack:"open"`
	Close     float64 `json:"close" msgpack:"close"`
}

// Portfolio represents portfolio data optimized for serialization
type Portfolio struct {
	ID            string      `json:"id" msgpack:"id"`
	TotalValue    float64     `json:"total_value" msgpack:"total_value"`
	Cash          float64     `json:"cash" msgpack:"cash"`
	UnrealizedPnL float64     `json:"unrealized_pnl" msgpack:"unrealized_pnl"`
	RealizedPnL   float64     `json:"realized_pnl" msgpack:"realized_pnl"`
	Positions     []Position  `json:"positions" msgpack:"positions"`
	LastUpdated   int64       `json:"last_updated" msgpack:"last_updated"`
}

type Position struct {
	Symbol         string  `json:"symbol" msgpack:"symbol"`
	Quantity       float64 `json:"quantity" msgpack:"quantity"`
	AvgCost        float64 `json:"avg_cost" msgpack:"avg_cost"`
	CurrentPrice   float64 `json:"current_price" msgpack:"current_price"`
	UnrealizedPnL  float64 `json:"unrealized_pnl" msgpack:"unrealized_pnl"`
	MarketValue    float64 `json:"market_value" msgpack:"market_value"`
}

// API Response structures
type APIResponse struct {
	Success bool        `json:"success" msgpack:"success"`
	Data    interface{} `json:"data,omitempty" msgpack:"data,omitempty"`
	Error   string      `json:"error,omitempty" msgpack:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty" msgpack:"meta,omitempty"`
}

type Meta struct {
	Timestamp    int64  `json:"timestamp" msgpack:"timestamp"`
	RequestID    string `json:"request_id" msgpack:"request_id"`
	Version      string `json:"version" msgpack:"version"`
	ResponseTime string `json:"response_time" msgpack:"response_time"`
}

// SuccessResponse creates a standardized success response
func SuccessResponse(data interface{}, meta *Meta) APIResponse {
	return APIResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
	}
}

// ErrorResponse creates a standardized error response
func ErrorResponse(err string, meta *Meta) APIResponse {
	return APIResponse{
		Success: false,
		Error:   err,
		Meta:    meta,
	}
}

// Utility functions for format conversion
func JSONToMessagePack(jsonData []byte) ([]byte, error) {
	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}
	return msgpack.Marshal(data)
}

func MessagePackToJSON(msgpackData []byte) ([]byte, error) {
	var data interface{}
	if err := msgpack.Unmarshal(msgpackData, &data); err != nil {
		return nil, err
	}
	return json.Marshal(data)
}