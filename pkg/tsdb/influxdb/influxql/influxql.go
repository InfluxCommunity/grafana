package influxql

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"go.opentelemetry.io/otel/trace"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/tsdb/influxdb/influxql/buffered"
	"github.com/grafana/grafana/pkg/tsdb/influxdb/influxql/querydata"
	"github.com/grafana/grafana/pkg/tsdb/influxdb/models"
	"github.com/grafana/grafana/pkg/tsdb/prometheus/utils"
)

const defaultRetentionPolicy = "default"

var (
	ErrInvalidHttpMode = errors.New("'httpMode' should be either 'GET' or 'POST'")
	glog               = log.New("tsdb.influx_influxql")
)

func Query(ctx context.Context, tracer trace.Tracer, dsInfo *models.DatasourceInfo, req *backend.QueryDataRequest, features featuremgmt.FeatureToggles) (*backend.QueryDataResponse, error) {
	logger := glog.FromContext(ctx)
	// Override the context for the logger temporarily

	response := backend.NewQueryDataResponse()

	for _, reqQuery := range req.Queries {
		query, err := models.QueryParse(reqQuery)
		if err != nil {
			return &backend.QueryDataResponse{}, err
		}

		rawQuery, err := query.Build(req)
		if err != nil {
			return &backend.QueryDataResponse{}, err
		}

		query.RefID = reqQuery.RefID
		query.RawQuery = rawQuery

		if setting.Env == setting.Dev {
			logger.Info("Influxdb query", "raw query", rawQuery)
		}

		request, err := createRequest(ctx, logger, dsInfo, rawQuery, query.Policy)
		if err != nil {
			return &backend.QueryDataResponse{}, err
		}

		resp, err := execute(ctx, tracer, dsInfo, logger, query, request, features.IsEnabled(ctx, featuremgmt.FlagInfluxqlStreamingParser))

		if err != nil {
			response.Responses[query.RefID] = backend.DataResponse{Error: err}
		} else {
			response.Responses[query.RefID] = resp
		}
	}

	return response, nil
}

func createRequest(ctx context.Context, logger log.Logger, dsInfo *models.DatasourceInfo, queryStr string, retentionPolicy string) (*http.Request, error) {
	u, err := url.Parse(dsInfo.URL)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(u.Path, "query")
	httpMode := dsInfo.HTTPMode

	var req *http.Request
	switch httpMode {
	case "GET":
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
	case "POST":
		bodyValues := url.Values{}
		bodyValues.Add("q", queryStr)
		body := bodyValues.Encode()
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(body))
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidHttpMode
	}

	params := req.URL.Query()
	params.Set("db", dsInfo.DbName)
	params.Set("epoch", "ms")
	// default is hardcoded default retention policy
	// InfluxDB will use the default policy when it is not added to the request
	if retentionPolicy != "" && retentionPolicy != "default" {
		params.Set("rp", retentionPolicy)
	}

	if httpMode == "GET" {
		params.Set("q", queryStr)
	} else if httpMode == "POST" {
		req.Header.Set("Content-type", "application/x-www-form-urlencoded")
	}

	req.URL.RawQuery = params.Encode()

	logger.Debug("Influxdb request", "url", req.URL.String())
	return req, nil
}

func execute(ctx context.Context, tracer trace.Tracer, dsInfo *models.DatasourceInfo, logger log.Logger, query *models.Query, request *http.Request, isStreamingParserEnabled bool) (backend.DataResponse, error) {
	res, err := dsInfo.HTTPClient.Do(request)
	if err != nil {
		return backend.DataResponse{}, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", "err", err)
		}
	}()

	_, endSpan := utils.StartTrace(ctx, tracer, "datasource.influxdb.influxql.parseResponse")
	defer endSpan()

	var resp *backend.DataResponse
	if isStreamingParserEnabled {
		logger.Info("InfluxDB InfluxQL streaming parser enabled: ", "info")
		resp = querydata.ResponseParse(res.Body, res.StatusCode, query)
	} else {
		resp = buffered.ResponseParse(res.Body, res.StatusCode, query)
	}
	return *resp, nil
}
