// Types for the relationship-capsule / 养熟 approval loop (Persistent Identity
// P1/P1-b). Mirrors the Go handler contract in internal/transport/profiles_handler.go.

export interface CapsuleSummary {
  relationship_key: string;
  status: string;
  owner_dog: string;
  source_ref: string;
  eval_approvals: number;
  eval_rejections: number;
  updated_at: number;
  pending_proposal: boolean;
}

export interface RelationshipCapsule {
  relationship_key: string;
  owner_dog: string;
  status: string;
  source_ref: string;
  correction_path?: string;
  body: string;
  eval_approvals: number;
  eval_rejections: number;
  updated_at: number;
  pending_proposal?: boolean;
}

export interface EvidenceItem {
  id: string;
  type: string;
  title: string;
  content: string;
  tags: string[];
  created_at: number;
}

export interface DistillResult {
  relationship_key: string;
  evidence_count: number;
  evidence: EvidenceItem[];
}

export interface DistillAgentResult {
  status: string;
  relationship_key: string;
  distiller: string;
  client: string;
  session_id?: string;
  evidence_count: number;
}
