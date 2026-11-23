export type HitMeta = {
  id: number;
  score: number;
  pillar: string;
  section: string;
  source: string;
};

export type Proposal = {
  summary: string;
  components: { name: string; role: string }[];
  mermaid: string;
  rationale: { pillar: string; text: string; chunk_ids: number[] }[];
  concerns: string[];
  questions: string[];
};

export type Chunk = {
  id: number;
  source: string;
  pillar: string;
  section: string;
  content: string;
};
