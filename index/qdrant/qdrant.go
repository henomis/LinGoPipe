package qdrant

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/henomis/lingoose/document"
	"github.com/henomis/lingoose/embedder"
	"github.com/henomis/lingoose/index"
	"github.com/henomis/lingoose/index/option"
	qdrantgo "github.com/henomis/qdrant-go"
	qdrantrequest "github.com/henomis/qdrant-go/request"
	qdrantresponse "github.com/henomis/qdrant-go/response"
)

const (
	defaultTopK            = 10
	defaultBatchUpsertSize = 32
)

type Index struct {
	qdrantClient    *qdrantgo.Client
	collectionName  string
	embedder        index.Embedder
	includeContent  bool
	batchUpsertSize int

	createCollection *CreateCollectionOptions
}

type Distance string

const (
	DistanceCosine    Distance = Distance(qdrantrequest.DistanceCosine)
	DistanceEuclidean Distance = Distance(qdrantrequest.DistanceEuclidean)
	DistanceDot       Distance = Distance(qdrantrequest.DistanceDot)
)

type CreateCollectionOptions struct {
	Dimension uint64
	Distance  Distance
	OnDisk    bool
}

type Options struct {
	CollectionName   string
	IncludeContent   bool
	BatchUpsertSize  *int
	CreateCollection *CreateCollectionOptions
}

func New(options Options, embedder index.Embedder) *Index {

	apiKey := os.Getenv("QDRANT_API_KEY")
	endpoint := os.Getenv("QDRANT_ENDPOINT")

	qdrantClient := qdrantgo.New(endpoint, apiKey)

	batchUpsertSize := defaultBatchUpsertSize
	if options.BatchUpsertSize != nil {
		batchUpsertSize = *options.BatchUpsertSize
	}

	return &Index{
		qdrantClient:     qdrantClient,
		collectionName:   options.CollectionName,
		embedder:         embedder,
		includeContent:   options.IncludeContent,
		batchUpsertSize:  batchUpsertSize,
		createCollection: options.CreateCollection,
	}
}

func (q *Index) WithAPIKeyAndEdpoint(apiKey, endpoint string) *Index {
	q.qdrantClient = qdrantgo.New(endpoint, apiKey)
	return q
}

func (q *Index) LoadFromDocuments(ctx context.Context, documents []document.Document) error {

	err := q.createCollectionIfRequired(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", index.ErrInternal, err)
	}

	err = q.batchUpsert(ctx, documents)
	if err != nil {
		return fmt.Errorf("%s: %w", index.ErrInternal, err)
	}
	return nil
}

func (q *Index) IsEmpty(ctx context.Context) (bool, error) {

	err := q.createCollectionIfRequired(ctx)
	if err != nil {
		return true, fmt.Errorf("%s: %w", index.ErrInternal, err)
	}

	res := &qdrantresponse.CollectionCollectInfo{}
	err = q.qdrantClient.CollectionCollectInfo(
		ctx,
		&qdrantrequest.CollectionCollectInfo{
			CollectionName: q.collectionName,
		},
		res,
	)
	if err != nil {
		return true, fmt.Errorf("%s: %w", index.ErrInternal, err)
	}

	return res.Result.VectorsCount == 0, nil

}

func (q *Index) Add(ctx context.Context, item *index.Data) error {
	err := q.createCollectionIfRequired(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", index.ErrInternal, err)
	}

	if item.ID == "" {
		id, err := uuid.NewUUID()
		if err != nil {
			return err
		}
		item.ID = id.String()
	}

	return q.pointUpsert(ctx,
		[]qdrantrequest.Point{
			{
				ID:      item.ID,
				Vector:  item.Values,
				Payload: item.Metadata,
			},
		},
	)
}

func (q *Index) Search(ctx context.Context, values []float64, opts ...option.Option) (index.SearchResults, error) {
	qdrantOptions := &option.Options{
		TopK: defaultTopK,
	}

	for _, opt := range opts {
		opt(qdrantOptions)
	}

	matches, err := q.similaritySearch(ctx, values, qdrantOptions)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", index.ErrInternal, err)
	}

	searchResults := buildSearchResultsFromQdrantMatches(matches, q.includeContent)

	return index.FilterSearchResults(searchResults, qdrantOptions.TopK), nil
}

func (q *Index) Query(ctx context.Context, query string, opts ...option.Option) (index.SearchResults, error) {

	qdrantOptions := &option.Options{
		TopK: defaultTopK,
	}

	for _, opt := range opts {
		opt(qdrantOptions)
	}

	matches, err := q.query(ctx, query, qdrantOptions)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", index.ErrInternal, err)
	}

	searchResults := buildSearchResultsFromQdrantMatches(matches, q.includeContent)

	return index.FilterSearchResults(searchResults, qdrantOptions.TopK), nil
}

func (q *Index) similaritySearch(
	ctx context.Context,
	values []float64,
	opts *option.Options,
) ([]qdrantresponse.PointSearchResult, error) {

	if opts.Filter == nil {
		opts.Filter = qdrantrequest.Filter{}
	}

	includeMetadata := true
	res := &qdrantresponse.PointSearch{}
	err := q.qdrantClient.PointSearch(
		ctx,
		&qdrantrequest.PointSearch{
			CollectionName: q.collectionName,
			Limit:          opts.TopK,
			Vector:         values,
			WithPayload:    &includeMetadata,
			Filter:         opts.Filter.(qdrantrequest.Filter),
		},
		res,
	)
	if err != nil {
		return nil, err
	}

	return res.Result, nil
}

func (q *Index) query(
	ctx context.Context,
	query string,
	opts *option.Options,
) ([]qdrantresponse.PointSearchResult, error) {
	embeddings, err := q.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}

	return q.similaritySearch(ctx, embeddings[0], opts)
}

func (q *Index) createCollectionIfRequired(ctx context.Context) error {

	if q.createCollection == nil {
		return nil
	}

	resp := &qdrantresponse.CollectionList{}
	err := q.qdrantClient.CollectionList(ctx, &qdrantrequest.CollectionList{}, resp)
	if err != nil {
		return err
	}

	for _, collection := range resp.Result.Collections {
		if collection.Name == q.collectionName {
			return nil
		}
	}

	req := &qdrantrequest.CollectionCreate{
		CollectionName: q.collectionName,
		Vectors: qdrantrequest.VectorsParams{
			Size:     q.createCollection.Dimension,
			Distance: qdrantrequest.Distance(q.createCollection.Distance),
			OnDisk:   &q.createCollection.OnDisk,
		},
	}

	err = q.qdrantClient.CollectionCreate(ctx, req, &qdrantresponse.CollectionCreate{})
	if err != nil {
		return err
	}

	return nil
}

func (q *Index) batchUpsert(ctx context.Context, documents []document.Document) error {

	for i := 0; i < len(documents); i += q.batchUpsertSize {

		batchEnd := i + q.batchUpsertSize
		if batchEnd > len(documents) {
			batchEnd = len(documents)
		}

		texts := []string{}
		for _, document := range documents[i:batchEnd] {
			texts = append(texts, document.Content)
		}

		embeddings, err := q.embedder.Embed(ctx, texts)
		if err != nil {
			return err
		}

		points, err := buildQdrantPointsFromEmbeddingsAndDocuments(embeddings, documents, i, q.includeContent)
		if err != nil {
			return err
		}

		err = q.pointUpsert(ctx, points)
		if err != nil {
			return err
		}
	}

	return nil
}

func (q *Index) pointUpsert(ctx context.Context, points []qdrantrequest.Point) error {

	wait := true
	req := &qdrantrequest.PointUpsert{
		Wait:           &wait,
		CollectionName: q.collectionName,
		Points:         points,
	}
	res := &qdrantresponse.PointUpsert{}

	err := q.qdrantClient.PointUpsert(ctx, req, res)
	if err != nil {
		return err
	}

	return nil
}

func buildQdrantPointsFromEmbeddingsAndDocuments(
	embeddings []embedder.Embedding,
	documents []document.Document,
	startIndex int,
	includeContent bool,
) ([]qdrantrequest.Point, error) {

	var vectors []qdrantrequest.Point

	for i, embedding := range embeddings {

		metadata := index.DeepCopyMetadata(documents[startIndex+i].Metadata)

		// inject document content into vector metadata
		if includeContent {
			metadata[index.DefaultKeyContent] = documents[startIndex+i].Content
		}

		vectorID, err := uuid.NewUUID()
		if err != nil {
			return nil, err
		}

		vectors = append(vectors, qdrantrequest.Point{
			ID:      vectorID.String(),
			Vector:  embedding,
			Payload: metadata,
		})

		// inject vector ID into document metadata
		documents[startIndex+i].Metadata[index.DefaultKeyID] = vectorID.String()
	}

	return vectors, nil
}

func buildSearchResultsFromQdrantMatches(
	matches []qdrantresponse.PointSearchResult,
	includeContent bool,
) index.SearchResults {
	searchResults := make([]index.SearchResult, len(matches))

	for i, match := range matches {

		metadata := index.DeepCopyMetadata(match.Payload)
		if !includeContent {
			delete(metadata, index.DefaultKeyContent)
		}

		searchResults[i] = index.SearchResult{
			Data: index.Data{
				ID:       match.ID,
				Metadata: metadata,
				Values:   match.Vector,
			},
			Score: match.Score,
		}
	}

	return searchResults
}
