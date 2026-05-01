package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/bulk"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/paxaf/itkFinal/gw-analytics/internal/domain"
	"github.com/stretchr/testify/suite"
)

var errBoom = errors.New("boom")

type MethodsSuite struct {
	suite.Suite

	ctx     context.Context
	client  *fakeClient
	bulk    *fakeBulkRequest
	storage *Storage
}

func TestMethodsSuite(t *testing.T) {
	suite.Run(t, new(MethodsSuite))
}

func (s *MethodsSuite) SetupTest() {
	s.ctx = context.Background()
	s.bulk = &fakeBulkRequest{}
	s.client = &fakeClient{bulk: s.bulk}
	s.storage = &Storage{client: s.client, index: "wallet_operations"}
}

func (s *MethodsSuite) TestSaveOperation() {
	event := validOperationEvent("event-1")

	err := s.storage.SaveOperation(s.ctx, event)

	s.Require().NoError(err)
	s.Require().Equal("wallet_operations", s.client.newBulkIndex)
	s.Require().True(s.bulk.doCalled)
	s.Require().Len(s.bulk.calls, 1)
	s.Require().Equal("event-1", *s.bulk.calls[0].op.Id_)
	s.requireUpdateAction(s.bulk.calls[0].action, event)
}

func (s *MethodsSuite) TestSaveOperations() {
	event1 := validOperationEvent("event-1")
	event2 := validOperationEvent("event-2")

	err := s.storage.SaveOperations(s.ctx, []domain.OperationEvent{event1, event2})

	s.Require().NoError(err)
	s.Require().Equal("wallet_operations", s.client.newBulkIndex)
	s.Require().True(s.bulk.doCalled)
	s.Require().Len(s.bulk.calls, 2)
	s.Require().Equal("event-1", *s.bulk.calls[0].op.Id_)
	s.Require().Equal("event-2", *s.bulk.calls[1].op.Id_)
}

func (s *MethodsSuite) TestSaveOperationsSkipsEmptyBatch() {
	err := s.storage.SaveOperations(s.ctx, nil)

	s.Require().NoError(err)
	s.Require().False(s.bulk.doCalled)
	s.Require().Empty(s.bulk.calls)
}

func (s *MethodsSuite) TestSaveOperationsReturnsUpdateOpError() {
	s.bulk.updateErr = errBoom

	err := s.storage.SaveOperations(s.ctx, []domain.OperationEvent{validOperationEvent("event-1")})

	s.Require().ErrorIs(err, errBoom)
	s.Require().False(s.bulk.doCalled)
}

func (s *MethodsSuite) TestSaveOperationsReturnsBulkError() {
	s.bulk.doErr = errBoom

	err := s.storage.SaveOperations(s.ctx, []domain.OperationEvent{validOperationEvent("event-1")})

	s.Require().ErrorIs(err, errBoom)
}

func (s *MethodsSuite) TestSaveOperationsReturnsItemErrors() {
	s.bulk.response = &bulk.Response{Errors: true}

	err := s.storage.SaveOperations(s.ctx, []domain.OperationEvent{validOperationEvent("event-1")})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "elastic returned item errors")
}

func (s *MethodsSuite) TestClose() {
	s.Require().NoError(s.storage.Close(s.ctx))
}

func (s *MethodsSuite) TestNewOperationDocument() {
	createdAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	deliveredAt := createdAt.Add(1500 * time.Millisecond)
	event := validOperationEvent("event-1")
	event.CreatedAt = createdAt

	doc := newOperationDocument(event, deliveredAt)

	s.Require().Equal(event.EventID, doc.EventID)
	s.Require().Equal(event.UserID, doc.UserID)
	s.Require().Equal(event.OperationType, doc.OperationType)
	s.Require().Equal(event.Status, doc.Status)
	s.Require().Equal(event.Currency, doc.Currency)
	s.Require().Equal(event.AmountMinor, doc.AmountMinor)
	s.Require().Equal(event.AmountRubMinor, doc.AmountRubMinor)
	s.Require().Equal(event.CreatedAt, doc.CreatedAt)
	s.Require().Equal(deliveredAt, doc.DeliveredAt)
	s.Require().Equal(int64(1500), doc.LatencyMS)
	s.Require().Equal(1, doc.DeliveryCount)
	s.Require().Zero(doc.DuplicateCount)
}

func (s *MethodsSuite) TestNewOperationDocumentClampsNegativeLatency() {
	createdAt := time.Date(2026, 5, 1, 12, 0, 1, 0, time.UTC)
	event := validOperationEvent("event-1")
	event.CreatedAt = createdAt

	doc := newOperationDocument(event, createdAt.Add(-time.Second))

	s.Require().Zero(doc.LatencyMS)
}

func (s *MethodsSuite) requireUpdateAction(action *types.UpdateAction, event domain.OperationEvent) {
	s.Require().NotNil(action)
	s.Require().NotNil(action.Script)
	s.Require().Equal(updateDeliveryStatsScript, action.Script.Source)
	s.Require().NotEmpty(action.Upsert)

	var doc operationDocument
	s.Require().NoError(json.Unmarshal(action.Upsert, &doc))
	s.Require().Equal(event.EventID, doc.EventID)
	s.Require().Equal(event.UserID, doc.UserID)
	s.Require().Equal(event.OperationType, doc.OperationType)
	s.Require().Equal(event.Status, doc.Status)
	s.Require().Equal(event.Currency, doc.Currency)
	s.Require().Equal(event.AmountMinor, doc.AmountMinor)
	s.Require().Equal(event.AmountRubMinor, doc.AmountRubMinor)
	s.Require().Equal(1, doc.DeliveryCount)
	s.Require().Zero(doc.DuplicateCount)
}

type ConnectorSuite struct {
	suite.Suite

	ctx     context.Context
	client  *fakeClient
	storage *Storage
}

func TestConnectorSuite(t *testing.T) {
	suite.Run(t, new(ConnectorSuite))
}

func (s *ConnectorSuite) SetupTest() {
	s.ctx = context.Background()
	s.client = &fakeClient{}
	s.storage = &Storage{client: s.client, index: "wallet_operations"}
}

func (s *ConnectorSuite) TestEnsureIndexSkipsExistingIndex() {
	s.client.exists = true

	err := s.storage.ensureIndex(s.ctx)

	s.Require().NoError(err)
	s.Require().False(s.client.createCalled)
}

func (s *ConnectorSuite) TestEnsureIndexCreatesMissingIndex() {
	err := s.storage.ensureIndex(s.ctx)

	s.Require().NoError(err)
	s.Require().True(s.client.createCalled)
	s.Require().Equal("wallet_operations", s.client.createdIndex)
	s.Require().NotNil(s.client.createdMapping)
}

func (s *ConnectorSuite) TestEnsureIndexReturnsExistsError() {
	s.client.existsErr = errBoom

	err := s.storage.ensureIndex(s.ctx)

	s.Require().ErrorIs(err, errBoom)
	s.Require().False(s.client.createCalled)
}

func (s *ConnectorSuite) TestEnsureIndexReturnsCreateError() {
	s.client.createErr = errBoom

	err := s.storage.ensureIndex(s.ctx)

	s.Require().ErrorIs(err, errBoom)
}

func validOperationEvent(eventID string) domain.OperationEvent {
	return domain.OperationEvent{
		EventID:        eventID,
		UserID:         42,
		OperationType:  "DEPOSIT",
		Status:         "SUCCESS",
		Currency:       "RUB",
		AmountMinor:    5_000_000,
		AmountRubMinor: 5_000_000,
		CreatedAt:      time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
	}
}

type fakeClient struct {
	exists         bool
	existsErr      error
	createErr      error
	createCalled   bool
	createdIndex   string
	createdMapping *types.TypeMapping
	bulk           *fakeBulkRequest
	newBulkIndex   string
}

func (f *fakeClient) IndexExists(_ context.Context, _ string) (bool, error) {
	return f.exists, f.existsErr
}

func (f *fakeClient) CreateIndex(_ context.Context, index string, mapping *types.TypeMapping) error {
	f.createCalled = true
	f.createdIndex = index
	f.createdMapping = mapping
	return f.createErr
}

func (f *fakeClient) NewBulk(index string) bulkRequest {
	f.newBulkIndex = index
	if f.bulk == nil {
		f.bulk = &fakeBulkRequest{}
	}
	return f.bulk
}

type updateCall struct {
	op     types.UpdateOperation
	doc    interface{}
	action *types.UpdateAction
}

type fakeBulkRequest struct {
	calls     []updateCall
	updateErr error
	doErr     error
	response  *bulk.Response
	doCalled  bool
}

func (f *fakeBulkRequest) UpdateOp(op types.UpdateOperation, doc interface{}, action *types.UpdateAction) error {
	if f.updateErr != nil {
		return f.updateErr
	}

	f.calls = append(f.calls, updateCall{
		op:     op,
		doc:    doc,
		action: action,
	})
	return nil
}

func (f *fakeBulkRequest) Do(_ context.Context) (*bulk.Response, error) {
	f.doCalled = true
	if f.doErr != nil {
		return nil, f.doErr
	}
	if f.response != nil {
		return f.response, nil
	}
	return &bulk.Response{}, nil
}
