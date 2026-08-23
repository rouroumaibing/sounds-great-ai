// FT-DS-001: capability dossier types (mirrors internal/dossier wire format).

export interface DossierProvenance {
  version: string;
  date: string;
  primarySources?: string[];
}

export interface RoutingSignals {
  peakCapabilities?: string[];
  antiSignals?: string[];
}

export interface DossierProfile {
  dogId: string;
  oneLiner?: string;
  l0RosterSummary?: string;
  l0RoutingNote?: string;
  routingSignals?: RoutingSignals;
  provenance?: DossierProvenance;
}

export interface DossierDogCard {
  dogId: string;
  breedId: string;
  displayName: string;
  variantId?: string;
  channel?: string;
  dossier: DossierProfile | null;
}

export interface DossierModelGroup {
  model: string;
  dogs: DossierDogCard[];
}

export interface DossierOverviewMeta {
  totalDogs: number;
  totalModels: number;
  dossierCoverage: number;
  dossierAvailable: boolean;
}

export interface DossierOverview {
  modelGroups: DossierModelGroup[];
  meta: DossierOverviewMeta;
}

export interface DossierObservation {
  id: string;
  dogId: string;
  content: string;
  provenance: {
    type: string;
    author: string;
    date: string;
  };
  createdAt: string;
}

export type DistillationProposalStatus = 'pending' | 'approved' | 'rejected' | 'applied';

// Transient workflow signal: "a capability-relevant event just closed;
// consider distilling it into the dossier". Deliberately not persisted —
// opportunities are prompts, not ledgers.
export interface DistillationOpportunity {
  opportunityId: string;
  sourceEvent: string;
  sourceId: string;
  targetDogId: string;
  threadId: string;
  reviewerDogId: string;
  authorDogId: string;
  status: 'pending' | 'converted' | 'dismissed';
  createdAt: string;
  convertedToProposalId?: string;
}

export interface EvidenceRef {
  type: string;
  id: string;
  summary?: string;
}

export interface DistillationProposal {
  proposalId: string;
  status: DistillationProposalStatus;
  sourceEvent: string;
  sourceId: string;
  targetDogId: string;
  targetFields: string[];
  beforeSnapshot: string;
  afterDraft: string;
  rationale: string;
  evidenceRefs: EvidenceRef[];
  baseHash: string;
  createdAt: string;
  createdBy: string;
  approvedBy?: string;
  rejectedBy?: string;
  rejectReason?: string;
  appliedBy?: string;
  appliedCommitSha?: string;
}
