package testutil

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudwego/eino/schema"

	"sounds-great-ai/internal/adapter/unified"
	"sounds-great-ai/internal/cue"
	"sounds-great-ai/internal/eval"
	"sounds-great-ai/internal/hooks"
	"sounds-great-ai/internal/memory"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/internal/threadstore"
	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/pack/orchestrator"
	"sounds-great-ai/pkg/protocol"
)

// --- MockVectorStore (ragstore.VectorStore) ---

type MockVectorStore struct {
	UpsertFn  func(ctx context.Context, docs []*schema.Document) error
	SearchFn  func(ctx context.Context, query string, opts ragstore.SearchOpts) ([]*schema.Document, error)
	DeleteFn  func(ctx context.Context, ids []string) error
	CloseFn   func() error
	ListAllFn func(ctx context.Context) ([]*schema.Document, error)
	GetByIDFn func(ctx context.Context, id string) (*schema.Document, error)
	DropAllFn func(ctx context.Context) error
}

func (m *MockVectorStore) Upsert(ctx context.Context, docs []*schema.Document) error {
	if m.UpsertFn != nil {
		return m.UpsertFn(ctx, docs)
	}
	return nil
}

func (m *MockVectorStore) Search(ctx context.Context, query string, opts ragstore.SearchOpts) ([]*schema.Document, error) {
	if m.SearchFn != nil {
		return m.SearchFn(ctx, query, opts)
	}
	return nil, nil
}

func (m *MockVectorStore) Delete(ctx context.Context, ids []string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, ids)
	}
	return nil
}

func (m *MockVectorStore) Close() error {
	if m.CloseFn != nil {
		return m.CloseFn()
	}
	return nil
}

func (m *MockVectorStore) ListAll(ctx context.Context) ([]*schema.Document, error) {
	if m.ListAllFn != nil {
		return m.ListAllFn(ctx)
	}
	return nil, nil
}

func (m *MockVectorStore) GetByID(ctx context.Context, id string) (*schema.Document, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *MockVectorStore) DropAll(ctx context.Context) error {
	if m.DropAllFn != nil {
		return m.DropAllFn(ctx)
	}
	return nil
}

// --- MockAgentExecutor (unified.AgentExecutor) ---

type MockAgentExecutor struct {
	ExecuteFn      func(ctx context.Context, req unified.ExecuteRequest) (<-chan unified.StreamEvent, error)
	CapabilitiesFn func() unified.AgentCapabilities
	HealthFn       func(ctx context.Context) error
}

func (m *MockAgentExecutor) Execute(ctx context.Context, req unified.ExecuteRequest) (<-chan unified.StreamEvent, error) {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(ctx, req)
	}
	return nil, nil
}

func (m *MockAgentExecutor) Capabilities() unified.AgentCapabilities {
	if m.CapabilitiesFn != nil {
		return m.CapabilitiesFn()
	}
	return unified.AgentCapabilities{}
}

func (m *MockAgentExecutor) Health(ctx context.Context) error {
	if m.HealthFn != nil {
		return m.HealthFn(ctx)
	}
	return nil
}

// --- MockThreadStore (threadstore.ThreadStore) ---

type MockThreadStore struct {
	CreateThreadFn  func(title string) (*threadstore.Thread, error)
	ListThreadsFn   func() ([]*threadstore.Thread, error)
	DeleteThreadFn  func(id string) error
	UpdateTitleFn   func(id string, title string) error
	AddEventFn      func(threadID string, event json.RawMessage) error
	GetEventsFn     func(threadID string) ([]json.RawMessage, error)
	CreateSessionFn func(threadID, breedID string) (*threadstore.SessionRecord, error)
	ListSessionsFn  func(threadID string) ([]*threadstore.SessionRecord, error)
	UnsealSessionFn func(sessionID string) error
}

func (m *MockThreadStore) CreateThread(title string) (*threadstore.Thread, error) {
	if m.CreateThreadFn != nil {
		return m.CreateThreadFn(title)
	}
	return nil, nil
}

func (m *MockThreadStore) ListThreads() ([]*threadstore.Thread, error) {
	if m.ListThreadsFn != nil {
		return m.ListThreadsFn()
	}
	return nil, nil
}

func (m *MockThreadStore) DeleteThread(id string) error {
	if m.DeleteThreadFn != nil {
		return m.DeleteThreadFn(id)
	}
	return nil
}

func (m *MockThreadStore) UpdateTitle(id string, title string) error {
	if m.UpdateTitleFn != nil {
		return m.UpdateTitleFn(id, title)
	}
	return nil
}

func (m *MockThreadStore) AddEvent(threadID string, event json.RawMessage) error {
	if m.AddEventFn != nil {
		return m.AddEventFn(threadID, event)
	}
	return nil
}

func (m *MockThreadStore) GetEvents(threadID string) ([]json.RawMessage, error) {
	if m.GetEventsFn != nil {
		return m.GetEventsFn(threadID)
	}
	return nil, nil
}

func (m *MockThreadStore) CreateSession(threadID, breedID string) (*threadstore.SessionRecord, error) {
	if m.CreateSessionFn != nil {
		return m.CreateSessionFn(threadID, breedID)
	}
	return nil, nil
}

func (m *MockThreadStore) ListSessions(threadID string) ([]*threadstore.SessionRecord, error) {
	if m.ListSessionsFn != nil {
		return m.ListSessionsFn(threadID)
	}
	return nil, nil
}

func (m *MockThreadStore) UnsealSession(sessionID string) error {
	if m.UnsealSessionFn != nil {
		return m.UnsealSessionFn(sessionID)
	}
	return nil
}

// --- MockSettingsStore (settings.SettingsStore) ---

type MockSettingsStore struct {
	ListMembersFn   func() ([]*settings.Member, error)
	CreateMemberFn  func(breedID, displayName, role string, enabled bool) (*settings.Member, error)
	UpdateMemberFn  func(id string, updates map[string]any) error
	DeleteMemberFn  func(id string) error
	ListAccountsFn  func() ([]*settings.Account, error)
	CreateAccountFn func(provider, apiKey string) (*settings.Account, error)
	DeleteAccountFn func(id string) error
	ListConfigFn    func() ([]*settings.SystemConfig, error)
	UpdateAccountFn func(id string, updates map[string]any) error
	UpdateConfigFn  func(key, value string) error
}

func (m *MockSettingsStore) ListMembers() ([]*settings.Member, error) {
	if m.ListMembersFn != nil {
		return m.ListMembersFn()
	}
	return nil, nil
}

func (m *MockSettingsStore) CreateMember(breedID, displayName, role string, enabled bool) (*settings.Member, error) {
	if m.CreateMemberFn != nil {
		return m.CreateMemberFn(breedID, displayName, role, enabled)
	}
	return nil, nil
}

func (m *MockSettingsStore) UpdateMember(id string, updates map[string]any) error {
	if m.UpdateMemberFn != nil {
		return m.UpdateMemberFn(id, updates)
	}
	return nil
}

func (m *MockSettingsStore) DeleteMember(id string) error {
	if m.DeleteMemberFn != nil {
		return m.DeleteMemberFn(id)
	}
	return nil
}

func (m *MockSettingsStore) ListAccounts() ([]*settings.Account, error) {
	if m.ListAccountsFn != nil {
		return m.ListAccountsFn()
	}
	return nil, nil
}

func (m *MockSettingsStore) CreateAccount(provider, apiKey string) (*settings.Account, error) {
	if m.CreateAccountFn != nil {
		return m.CreateAccountFn(provider, apiKey)
	}
	return nil, nil
}

func (m *MockSettingsStore) DeleteAccount(id string) error {
	if m.DeleteAccountFn != nil {
		return m.DeleteAccountFn(id)
	}
	return nil
}

func (m *MockSettingsStore) ListConfig() ([]*settings.SystemConfig, error) {
	if m.ListConfigFn != nil {
		return m.ListConfigFn()
	}
	return nil, nil
}

func (m *MockSettingsStore) UpdateAccount(id string, updates map[string]any) error {
	if m.UpdateAccountFn != nil {
		return m.UpdateAccountFn(id, updates)
	}
	return nil
}

func (m *MockSettingsStore) UpdateConfig(key, value string) error {
	if m.UpdateConfigFn != nil {
		return m.UpdateConfigFn(key, value)
	}
	return nil
}

// --- MockCredentialStore (settings.CredentialStore) ---

type MockCredentialStore struct {
	GetFn    func(accountID string) (string, error)
	SetFn    func(accountID, apiKey string) error
	DeleteFn func(accountID string) error
	HasFn    func(accountID string) bool
}

func (m *MockCredentialStore) Get(accountID string) (string, error) {
	if m.GetFn != nil {
		return m.GetFn(accountID)
	}
	return "", nil
}

func (m *MockCredentialStore) Set(accountID, apiKey string) error {
	if m.SetFn != nil {
		return m.SetFn(accountID, apiKey)
	}
	return nil
}

func (m *MockCredentialStore) Delete(accountID string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(accountID)
	}
	return nil
}

func (m *MockCredentialStore) Has(accountID string) bool {
	if m.HasFn != nil {
		return m.HasFn(accountID)
	}
	return false
}

// --- MockMessageStore (threadstore.MessageStore) ---

type MockMessageStore struct {
	AppendFn            func(msg *threadstore.Message) error
	GetByThreadFn       func(threadID string, limit int) ([]*threadstore.Message, error)
	GetByThreadBeforeFn func(threadID string, before time.Time, beforeID string, limit int) ([]*threadstore.Message, error)
}

func (m *MockMessageStore) Append(msg *threadstore.Message) error {
	if m.AppendFn != nil {
		return m.AppendFn(msg)
	}
	return nil
}

func (m *MockMessageStore) GetByThread(threadID string, limit int) ([]*threadstore.Message, error) {
	if m.GetByThreadFn != nil {
		return m.GetByThreadFn(threadID, limit)
	}
	return nil, nil
}

func (m *MockMessageStore) GetByThreadBefore(threadID string, before time.Time, beforeID string, limit int) ([]*threadstore.Message, error) {
	if m.GetByThreadBeforeFn != nil {
		return m.GetByThreadBeforeFn(threadID, before, beforeID, limit)
	}
	return nil, nil
}

// --- MockEvidenceStore (memory.EvidenceStore) ---

type MockEvidenceStore struct {
	ListEvidenceFn func() ([]*memory.EvidenceRecord, error)
	AddEvidenceFn  func(threadID, typ, title, content string, tags []string) (*memory.EvidenceRecord, error)
}

func (m *MockEvidenceStore) ListEvidence() ([]*memory.EvidenceRecord, error) {
	if m.ListEvidenceFn != nil {
		return m.ListEvidenceFn()
	}
	return nil, nil
}

func (m *MockEvidenceStore) AddEvidence(threadID, typ, title, content string, tags []string) (*memory.EvidenceRecord, error) {
	if m.AddEvidenceFn != nil {
		return m.AddEvidenceFn(threadID, typ, title, content, tags)
	}
	return nil, nil
}

// --- MockClosureService (eval.ClosureService) ---

type MockClosureService struct {
	AppendEventFn  func(ctx context.Context, verdictID string, event eval.ClosureEvent) error
	CurrentStateFn func(ctx context.Context, verdictID string) (eval.ClosureState, error)
	GetEventsFn    func(ctx context.Context, verdictID string) ([]eval.ClosureEvent, error)
}

func (m *MockClosureService) AppendEvent(ctx context.Context, verdictID string, event eval.ClosureEvent) error {
	if m.AppendEventFn != nil {
		return m.AppendEventFn(ctx, verdictID, event)
	}
	return nil
}

func (m *MockClosureService) CurrentState(ctx context.Context, verdictID string) (eval.ClosureState, error) {
	if m.CurrentStateFn != nil {
		return m.CurrentStateFn(ctx, verdictID)
	}
	return eval.ClosureState(""), nil
}

func (m *MockClosureService) GetEvents(ctx context.Context, verdictID string) ([]eval.ClosureEvent, error) {
	if m.GetEventsFn != nil {
		return m.GetEventsFn(ctx, verdictID)
	}
	return nil, nil
}

// --- MockResolver (hooks.Resolver) ---

type MockResolver struct {
	ResolveFn func(input *hooks.AssemblerInput) hooks.ResolveResult
}

func (m *MockResolver) Resolve(input *hooks.AssemblerInput) hooks.ResolveResult {
	if m.ResolveFn != nil {
		return m.ResolveFn(input)
	}
	return hooks.ResolveResult{}
}

// --- MockLaneResolver (cue.LaneResolver) ---

type MockLaneResolver struct {
	LaneFn    func() string
	ResolveFn func(ctx context.Context, subject, reason string, budget int) ([]cue.ResolvedCue, error)
}

func (m *MockLaneResolver) Lane() string {
	if m.LaneFn != nil {
		return m.LaneFn()
	}
	return ""
}

func (m *MockLaneResolver) Resolve(ctx context.Context, subject, reason string, budget int) ([]cue.ResolvedCue, error) {
	if m.ResolveFn != nil {
		return m.ResolveFn(ctx, subject, reason, budget)
	}
	return nil, nil
}

// --- MockCapability (pack.Capability) ---

type MockCapability struct {
	NameFn    func() string
	VersionFn func() string
	InitFn    func(ctx context.Context) error
	RunFn     func(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error)
	HealthFn  func() error
	CloseFn   func() error
}

func (m *MockCapability) Name() string {
	if m.NameFn != nil {
		return m.NameFn()
	}
	return ""
}

func (m *MockCapability) Version() string {
	if m.VersionFn != nil {
		return m.VersionFn()
	}
	return ""
}

func (m *MockCapability) Init(ctx context.Context) error {
	if m.InitFn != nil {
		return m.InitFn(ctx)
	}
	return nil
}

func (m *MockCapability) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	if m.RunFn != nil {
		return m.RunFn(ctx, input)
	}
	return nil, nil
}

func (m *MockCapability) Health() error {
	if m.HealthFn != nil {
		return m.HealthFn()
	}
	return nil
}

func (m *MockCapability) Close() error {
	if m.CloseFn != nil {
		return m.CloseFn()
	}
	return nil
}

// --- MockEventSink (pack.EventSink) ---

type MockEventSink struct {
	SendFn func(ctx context.Context, ev *protocol.Event) error
}

func (m *MockEventSink) Send(ctx context.Context, ev *protocol.Event) error {
	if m.SendFn != nil {
		return m.SendFn(ctx, ev)
	}
	return nil
}

// --- MockBreedExecutor (pack.BreedExecutor) ---

type MockBreedExecutor struct {
	ExecuteDispatchFn func(ctx context.Context, plan orchestrator.DispatchPlan, subtasks []orchestrator.SubTask) (results map[string]*pack.TaskOutput, entryErrors map[string]string, err error)
}

func (m *MockBreedExecutor) ExecuteDispatch(ctx context.Context, plan orchestrator.DispatchPlan, subtasks []orchestrator.SubTask) (results map[string]*pack.TaskOutput, entryErrors map[string]string, err error) {
	if m.ExecuteDispatchFn != nil {
		return m.ExecuteDispatchFn(ctx, plan, subtasks)
	}
	return nil, nil, nil
}
