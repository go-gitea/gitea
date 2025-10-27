<script setup lang="ts">
/* FishboneGraph.vue
   QUICK FIX SUMMARY:
   - We now transform the SAME <g> that Vue renders into (ref="worldRef").
   - We set `touch-action: none` on the <svg> so pinch zoom works on touch.
   - All interactions (pan/zoom/click/dblclick/alt+click/background reset)
     are wired to this world group through d3-zoom on the <svg>.
   Everything else remains as before (responsive dials, tiny-graph elegance). */

import { onMounted, reactive, ref, onBeforeUnmount, nextTick, computed } from "vue";
import { select } from "d3-selection";
import type { Selection } from "d3-selection";
import { zoom, zoomIdentity } from "d3-zoom";
import type { ZoomBehavior, ZoomTransform } from "d3-zoom";

import LegendFishbone from "./FishboneLegend.vue";
import BubbleNode from "./BubbleNode.vue";

// Inline types replacing former seeds module
type Side = -1 | 1;
type SeedKey = 'reference';
type NodeId = string;
type Node = {
  id: NodeId;
  contributors: number;
  parentId: NodeId | null;
  children: NodeId[];
  updatedAt?: string;
  sideHint?: Side;
  x?: number;
  y?: number;
  depth?: number;
  repoOwner?: string;
  repoName?: string;
  repoSubject?: string;
  fullName?: string;
};
type Graph = Record<string, Node>;

type RepoSelectionDetail = { owner: string; repo: string; subject?: string | null };

const LS_OWNER_KEY = 'selectedArticleOwner';
const LS_SUBJECT_KEY = 'selectedArticleSubject';
const LS_REPO_KEY = 'selectedArticleRepo';

/* ──────────────────────────────────────────────────────────────────────────────
   LAYOUT CONSTANTS (all values explained to avoid "magic numbers")
   ─────────────────────────────────────────────────────────────────────────── */

/* === DEMO DATA GENERATION === */
const RANDOM_MIN = 10;          // Minimum random contributor count for demo forks
const RANDOM_MAX = 600;         // Maximum random contributor count for demo forks

/* === BUBBLE SIZING === */
const R_MIN = 8;                // Minimum bubble radius in pixels (smallest contributor count)
const R_MAX = 120;              // Maximum bubble radius in pixels (largest contributor count)
const MAX_DEPTH = 4;            // Maximum fork depth to prevent exponential explosion in demos

/* === VERTICAL LAYOUT === */
const LEVEL_GAP = 240;          // Vertical spacing between generations (parent to child level)
const STEM_LEN_PARENT = 12;     // Short vertical stem extending from parent bubble
const STEM_LEN_CHILD  = 18;     // Short vertical stem extending to child bubble

/* === LAYOUT DEFAULTS (used in manual mode or as auto-tuning hints) === */
const BRANCH_SPACING_DEFAULT = 28;   // Default vertical gap between branch joints on trunk
const LANE_PAD_DEFAULT       = 12;   // Default padding between bubbles in same lane
const H_OFFSET_DEFAULT       = 48;   // Default horizontal rib length (parent to child)
const ELBOW_R_DEFAULT        = 28;   // Default elbow corner radius

/* === COLLISION CLEARANCES === */
const BUBBLE_PAD_DEFAULT = 8;   // Minimum clearance between bubbles
const PATH_PAD_DEFAULT   = 8;   // Minimum clearance between bubbles and paths

/* === ZOOM/PAN CONSTRAINTS === */
const ZOOM_MIN = 0.35;          // Minimum zoom level (35% scale)
const ZOOM_MAX = 3.5;           // Maximum zoom level (350% scale)

/* === VIEW RESET PARAMETERS === */
const RESET_TOP_MARGIN = 40;    // Top margin when resetting view to center content
const MAX_REF_DROP = 130;       // Maximum vertical drop for reference scenario to keep layout tight

/* === RESPONSIVE BREAKPOINTS & FACTORS === */
const WIDTH_BREAKPOINT_MIN = 480;    // Minimum container width for responsive calculations
const WIDTH_BREAKPOINT_MAX = 1200;   // Maximum container width for responsive calculations
const HEIGHT_BREAKPOINT_MIN = 640;   // Minimum container height for radius scaling
const HEIGHT_BREAKPOINT_MAX = 1080;  // Maximum container height for radius scaling
const COMPLEXITY_THRESHOLD = 10;     // Number of forks to reach full complexity factor
const FANOUT_THRESHOLD = 6;          // Number of children to reach full fanout factor

/* === RESPONSIVE H_OFFSET (horizontal rib length) === */
const H_OFFSET_MIN = 36;        // Minimum horizontal offset for narrow/simple graphs
const H_OFFSET_MAX = 84;        // Maximum horizontal offset for wide/complex graphs
const H_OFFSET_WIDTH_WEIGHT = 0.35;   // Weight of container width in h_offset calculation
const H_OFFSET_COMPLEXITY_WEIGHT = 0.65;  // Weight of complexity/fanout in h_offset calculation

/* === RESPONSIVE ELBOW_R (corner radius) === */
const ELBOW_MIN = 20;           // Minimum elbow radius
const ELBOW_MAX = 36;           // Maximum elbow radius
const ELBOW_RATIO = 0.55;       // Elbow radius as ratio of h_offset

/* === RESPONSIVE BRANCH_SPACING (vertical joint gap) === */
const BRANCH_SPACING_MIN = 24;  // Minimum vertical spacing between branch joints
const BRANCH_SPACING_MAX = 36;  // Maximum vertical spacing between branch joints
const BRANCH_SPACING_BASE_WEIGHT = 0.25;  // Base weight before width/complexity factors
const BRANCH_SPACING_FACTOR_WEIGHT = 0.75;  // Weight of width/complexity factors

/* === RESPONSIVE LANE_PAD (bubble clearance in lanes) === */
const LANE_PAD_BASE = 8;        // Base lane padding
const LANE_PAD_EXTRA = 12;      // Extra lane padding at maximum responsiveness
const LANE_PAD_WIDTH_WEIGHT = 0.5;    // Weight of width factor in lane padding
const LANE_PAD_COMPLEXITY_WEIGHT = 0.3;  // Weight of complexity factor in lane padding

/* === RADIUS SCALING (for vertical fit without scrolling) === */
const RADIUS_HEIGHT_MIN_FACTOR = 0.7;     // Minimum height-based radius multiplier
const RADIUS_HEIGHT_MAX_FACTOR = 1.0;     // Maximum height-based radius multiplier
// Calculate range from min to max for readability
const RADIUS_HEIGHT_RANGE_FACTOR = RADIUS_HEIGHT_MAX_FACTOR - RADIUS_HEIGHT_MIN_FACTOR;
const RADIUS_FORK_MAX_REDUCTION = 0.20;   // Maximum reduction from fork count (20%)
const RADIUS_FORK_STEP = 0.02;            // Reduction per fork (2% per fork)
const RADIUS_MIN_SCALE = 0.65;            // Minimum overall radius scale to avoid over-shrinking

/* === VIEW FITTING (reset/focus zoom calculations) === */
const FILL_FRACTION_MIN = 0.55;      // Minimum horizontal fill fraction for few forks
const FILL_FRACTION_MAX = 0.90;      // Maximum horizontal fill fraction for many forks
const VERTICAL_FILL_FRACTION = 0.86; // Vertical fill fraction of usable height
const SINGLE_FORK_DIAMETER_MIN = 220;  // Minimum bubble diameter for single-fork view
const SINGLE_FORK_DIAMETER_MAX = 480;  // Maximum bubble diameter for single-fork view
const SINGLE_FORK_WIDTH_RATIO = 0.40;  // Desired diameter as ratio of container width
const FOCUS_PADDING = 24;            // Padding when focusing on a single node

/* === SVG LAYOUT === */
const DEFAULT_SVG_HEIGHT = 1000;     // Initial SVG canvas height
const SVG_BOTTOM_PADDING = 240;      // Extra padding below lowest bubble
const CONTENT_BOUNDS_EXTRA = 16;     // Extra horizontal padding for elbow overhang
const VIEW_TOP_OFFSET = 12;          // Top offset for view calculations
const DEFAULT_CONTAINER_WIDTH = 1100;   // Default container width when not measured
const DEFAULT_CONTAINER_HEIGHT = 800;   // Default container height when not measured
const FALLBACK_WINDOW_HEIGHT = 900;     // Fallback height if window is unavailable

/* === API PARAMETERS === */
const API_CONTRIBUTOR_DAYS = 90;     // Number of days to look back for contributor counts
const API_MAX_DEPTH = 10;            // Maximum fork depth to fetch from API
const API_LIMIT = 50;                // Maximum number of forks to fetch per request

/* === ANIMATION DURATIONS === */
const VIEW_TRANSITION_DURATION = 420;  // Duration of zoom/pan animations in milliseconds
const SCREEN_READER_ANNOUNCEMENT_DURATION = 1000;  // How long to show SR announcements

/* === ID GENERATION === */
const RANDOM_ID_LENGTH = 7;          // Number of characters for generated random IDs
const RANDOM_ID_SLICE_START = 2;     // Starting position for ID slice (skip "0.")

/* ──────────────────────────────────────────────────────────────────────────────
   STATE
   ─────────────────────────────────────────────────────────────────────────── */
// NodeId defined above

const state = reactive({
  graph: {} as Graph,

  /* Scenario key */
  scenario: "reference" as SeedKey,

  /* Layout dials (manual when auto=false; hints when auto=true) */
  elbowR: ELBOW_R_DEFAULT,
  hOffset: H_OFFSET_DEFAULT,
  lanePad: LANE_PAD_DEFAULT,
  branchSpacing: BRANCH_SPACING_DEFAULT,
  bubblePad: BUBBLE_PAD_DEFAULT,
  pathPad: PATH_PAD_DEFAULT,

  auto: true,                             // responsive auto-tuning toggle
  /* Max contributors across current graph (for relative radius scaling) */
  maxContrib: 1,
  /* Additional global attenuation to reduce bubble sizes for small screens / many forks */
  radiusScale: 1,
});

/* Derived arrays used for Vue rendering (instead of D3 joins) */
type EdgeGeom = { source: Node; target: Node; side: Side;
  ex:number; ey:number; hx:number; hy:number; cx:number; cy:number; sx1:number; sy1:number; sx2:number; sy2:number; };
const nodesList = ref<Node[]>([]);
const edgesList = ref<EdgeGeom[]>([]);
const trunksList = ref<{ x:number; y1:number; y2:number; id:string }[]>([]);
const jointDots = ref<{ x:number; y:number; id:string }[]>([]);

/* SVG/zoom plumbing */
const svgHeight = ref(DEFAULT_SVG_HEIGHT);
const svgRef = ref<SVGSVGElement | null>(null);
/* IMPORTANT: This is the single world group that Vue renders into AND
   that d3-zoom transforms. This fixes the "graph doesn't move" bug. */
const worldRef = ref<SVGGElement | null>(null);

let svgSel!: Selection<SVGSVGElement, unknown, null, undefined>;
let worldSel!: Selection<SVGGElement, unknown, null, undefined>;
let zoomBehavior!: ZoomBehavior<Element, unknown>;
const currentK = ref(1);

/* Accessibility: Screen reader announcements */
const srAnnouncement = ref("");
function announceToScreenReader(message: string) {
  srAnnouncement.value = message;
  setTimeout(() => { srAnnouncement.value = ""; }, SCREEN_READER_ANNOUNCEMENT_DURATION);
}

/* Container width affects responsive dials; observe it. */
const containerRef = ref<HTMLDivElement | null>(null);
let ro: ResizeObserver | null = null;
let containerWidth = DEFAULT_CONTAINER_WIDTH;
let containerHeight = DEFAULT_CONTAINER_HEIGHT;
let pendingRaf: number | null = null;

/* Props and API fetch */
const props = defineProps<{ apiUrl?: string | null; owner?: string | null; repo?: string | null; subject?: string | null }>();

const selectedNodeId = ref<NodeId | null>(null);
let pendingExternalSelection: RepoSelectionDetail | null = null;

function normalize(value?: string | null) {
  return (value ?? '').toLowerCase();
}

function readStoredSelection(): RepoSelectionDetail | null {
  try {
    const owner = window.localStorage.getItem(LS_OWNER_KEY);
    const repo = window.localStorage.getItem(LS_REPO_KEY);
    const subject = window.localStorage.getItem(LS_SUBJECT_KEY);
    if (!owner) return null;
    if (repo) {
      return {owner, repo, subject: subject || null};
    }
    if (!subject) return null;
    return {owner, repo: subject, subject};
  } catch {
    return null;
  }
}

function getSelectionDetailFromNode(n: Node): RepoSelectionDetail | null {
  const ownerCandidates = [
    n.repoOwner,
    n.fullName?.split('/')?.[0],
    n.parentId === null ? (props.owner ?? null) : null,
  ].filter(Boolean) as string[];
  const repoCandidates = [
    n.repoName,
    n.fullName?.split('/')?.[1],
    n.repoSubject,
    n.parentId === null ? (props.repo ?? null) : null,
  ].filter(Boolean) as string[];
  const subjectCandidates = [
    n.repoSubject,
    n.repoName,
    n.fullName?.split('/')?.[1],
    n.parentId === null ? (props.subject ?? null) : null,
  ].filter(Boolean) as string[];

  const owner = ownerCandidates[0];
  const repo = repoCandidates[0] || subjectCandidates[0];
  if (!owner || !repo) return null;
  const subject = subjectCandidates[0] || null;
  return {owner, repo, subject};
}

function normalizeDetail(detail: RepoSelectionDetail | null): RepoSelectionDetail | null {
  if (!detail) return null;
  const repo = detail.repo || detail.subject || '';
  if (!detail.owner || !repo) return null;
  return {
    owner: detail.owner,
    repo,
    subject: detail.subject ?? detail.repo ?? null,
  };
}

function findNodeBySelection(detail: RepoSelectionDetail): Node | null {
  const desiredOwner = normalize(detail.owner);
  const desiredRepo = normalize(detail.repo || detail.subject || '');
  if (!desiredOwner || !desiredRepo) return null;
  for (const node of Object.values(state.graph)) {
    const ownerCandidates = [
      node.repoOwner,
      node.fullName?.split('/')?.[0],
      node.parentId === null ? (props.owner ?? null) : null,
    ].filter(Boolean) as string[];
    const repoCandidates = [
      node.repoName,
      node.fullName?.split('/')?.[1],
      node.repoSubject,
      node.parentId === null ? (props.repo ?? null) : null,
    ].filter(Boolean) as string[];
    if (
      ownerCandidates.some((c) => normalize(c) === desiredOwner) &&
      repoCandidates.some((c) => normalize(c) === desiredRepo)
    ) {
      return node;
    }
  }
  return null;
}

function applySelection(node: Node | null, _detail: RepoSelectionDetail | null) {
  selectedNodeId.value = node ? node.id : null;
}

function setSelectionFromDetail(detail: RepoSelectionDetail | null) {
  const normalized = normalizeDetail(detail);
  if (!normalized) {
    pendingExternalSelection = null;
    applySelection(null, null);
    return;
  }
  const node = findNodeBySelection(normalized);
  if (node) {
    pendingExternalSelection = null;
    applySelection(node, normalized);
  } else {
    pendingExternalSelection = normalized;
    applySelection(null, normalized);
  }
}

function restoreSelectionAfterGraphLoad() {
  const desired = pendingExternalSelection ?? readStoredSelection();
  if (desired) {
    pendingExternalSelection = null;
    setSelectionFromDetail(desired);
  } else {
    applySelection(null, null);
  }
}

function handleExternalSelection(event: Event) {
  const rawDetail = (event as CustomEvent<RepoSelectionDetail | null>).detail ?? null;
  const normalized = normalizeDetail(rawDetail);
  setSelectionFromDetail(normalized);
}

async function fetchForkGraphAndSet(){
  try{
    if(!props.apiUrl){
      console.warn('FishboneGraph: apiUrl not provided');
      return;
    }
    const urlObj = new URL(props.apiUrl, window.location.origin);
    if(!urlObj.searchParams.get('include_contributors')) urlObj.searchParams.set('include_contributors','true');
    if(!urlObj.searchParams.get('contributor_days')) urlObj.searchParams.set('contributor_days', API_CONTRIBUTOR_DAYS.toString());
    if(!urlObj.searchParams.get('max_depth')) urlObj.searchParams.set('max_depth', API_MAX_DEPTH.toString());
    if(!urlObj.searchParams.get('sort')) urlObj.searchParams.set('sort','updated');
    if(!urlObj.searchParams.get('limit')) urlObj.searchParams.set('limit', API_LIMIT.toString());

    const res = await fetch(urlObj.toString(), { credentials: 'same-origin' });
    if(!res.ok){ console.error('FishboneGraph: API error', res.status); return; }
    const json = await res.json();
    const graph = buildGraphFromApi(json?.root);
    state.graph = graph;
    layoutAndRender();
    resetView();
    restoreSelectionAfterGraphLoad();
  }catch(err){
    console.error('FishboneGraph: failed to fetch graph', err);
  }
}

function buildGraphFromApi(root:any): Graph{
  const g:Graph = {};
  if(!root) return g;
  const visit = (n:any, parentId: string | null)=>{
    const id: string = n?.id ?? (n?.repository?.full_name ?? Math.random().toString(36).slice(2));
    const baseContrib: number = Number(n?.contributors?.total_count ?? n?.contributors?.recent_count ?? 0);
    const contributors: number = Number.isFinite(baseContrib) ? baseContrib : 0;
    const updatedAt: string | undefined = n?.repository?.updated_at ?? n?.repository?.updated ?? undefined;
    const repo = n?.repository ?? {};
    const ownerName: string | null =
      repo?.owner?.name ?? repo?.owner_name ?? repo?.owner?.username ?? null;
    const repoName: string | null = repo?.name ?? repo?.repo_name ?? null;
    const repoSubject: string | null =
      repo?.subject ?? repo?.subject_slug ?? repo?.subject_name ?? repoName ?? null;
    const fullName: string | null = repo?.full_name ?? (ownerName && repoName ? `${ownerName}/${repoName}` : null);

    const node: Node = {
      id,
      contributors,
      parentId,
      children: [],
      updatedAt,
      repoOwner: ownerName ?? undefined,
      repoName: repoName ?? undefined,
      repoSubject: repoSubject ?? undefined,
      fullName: fullName ?? undefined,
    };
    if (!node.repoSubject && parentId === null && props.subject) {
      node.repoSubject = props.subject;
    }
    g[id] = node;
    for(const child of (n?.children ?? [])){
      const childId: string = child?.id ?? (child?.repository?.full_name ?? Math.random().toString(36).slice(2));
      node.children.push(childId);
      visit(child, id);
    }
  };
  visit(root, null);
  return g;
}

/* ──────────────────────────────────────────────────────────────────────────────
   HELPERS (math + graph)
   ─────────────────────────────────────────────────────────────────────────── */

function rFor(n:number){
  const max = state.maxContrib || 1;
  if (max <= 0) return R_MIN;
  const t = Math.max(0, Math.min(1, n / max));
  const base = R_MIN + (R_MAX - R_MIN) * t;
  return base * (state.radiusScale || 1);
}

function getRoot(g:Graph){ return Object.values(g).find(n=>n.parentId===null)!; }

function computeDepths(g:Graph){
  /* BFS depth tagging so we can place parents top-down and sort render order. */
  const root = getRoot(g) as any; (root as any).depth = 0;
  const q = [root];
  while(q.length){
    const n:any = q.shift();
    for(const cid of n.children){ const c:any = g[cid]; c.depth = (n.depth ?? 0) + 1; q.push(c); }
  }
}

function forkCount(g:Graph){ return Object.values(g).filter(n=>n.parentId!==null).length; }
function parentMaxChildren(g:Graph){ return Math.max(0, ...Object.values(g).map(n=>n.children.length)); }

/* ─────────────────────────────────────────────────────────────────────────────-
   RESPONSIVE AUTO-TUNING (adapts dials to width & complexity)
   ─────────────────────────────────────────────────────────────────────────── */
function applyResponsiveDials(){
  if (!state.auto) return;                 // manual mode: honor sliders
  const forks = forkCount(state.graph);
  const maxKids = parentMaxChildren(state.graph);
  const w = containerWidth;
  const ch = containerHeight || ((typeof window !== 'undefined' && window.innerHeight) ? window.innerHeight : FALLBACK_WINDOW_HEIGHT);

  // Normalize width to 0..1 range based on breakpoints
  const widthFactor = Math.min(1, Math.max(0, (w - WIDTH_BREAKPOINT_MIN) / (WIDTH_BREAKPOINT_MAX - WIDTH_BREAKPOINT_MIN)));
  // Normalize complexity based on fork count (0..1 over COMPLEXITY_THRESHOLD forks)
  const complexity  = Math.min(1, (forks / COMPLEXITY_THRESHOLD));
  // Normalize fanout based on children count (0..1 over FANOUT_THRESHOLD children)
  const fanout      = Math.min(1, (maxKids / FANOUT_THRESHOLD));

  // Calculate horizontal offset (rib length) using weighted combination
  const mix = H_OFFSET_WIDTH_WEIGHT * widthFactor + H_OFFSET_COMPLEXITY_WEIGHT * Math.max(complexity, fanout);
  state.hOffset = Math.round(H_OFFSET_MIN + (H_OFFSET_MAX - H_OFFSET_MIN) * mix);

  // Calculate elbow radius as a ratio of h_offset, clamped to min/max
  state.elbowR = Math.min(ELBOW_MAX, Math.max(ELBOW_MIN, Math.round(ELBOW_RATIO * state.hOffset)));

  // Calculate branch spacing (vertical joint gap)
  const branchFactor = BRANCH_SPACING_BASE_WEIGHT + BRANCH_SPACING_FACTOR_WEIGHT * Math.max(widthFactor, complexity);
  state.branchSpacing = Math.round(BRANCH_SPACING_MIN + (BRANCH_SPACING_MAX - BRANCH_SPACING_MIN) * branchFactor);

  // Calculate lane padding (bubble clearance)
  state.lanePad = Math.round(LANE_PAD_BASE + LANE_PAD_EXTRA * Math.max(widthFactor * LANE_PAD_WIDTH_WEIGHT, complexity * LANE_PAD_COMPLEXITY_WEIGHT));

  // Compute a gentle attenuation for bubble sizes to improve vertical fit without scrolling
  // Height factor: smaller screens → smaller bubbles
  const heightNorm = Math.min(1, Math.max(0, (ch - HEIGHT_BREAKPOINT_MIN) / (HEIGHT_BREAKPOINT_MAX - HEIGHT_BREAKPOINT_MIN)));
  const heightFactor = RADIUS_HEIGHT_MIN_FACTOR + RADIUS_HEIGHT_RANGE_FACTOR * heightNorm;
  // Fork factor: more forks → smaller bubbles
  const forksFactor = 1 - Math.min(RADIUS_FORK_MAX_REDUCTION, forks * RADIUS_FORK_STEP);
  // Combine and clamp to avoid over-shrinking
  state.radiusScale = Math.max(RADIUS_MIN_SCALE, Math.min(1, heightFactor * forksFactor));
}

/* ─────────────────────────────────────────────────────────────────────────────-
   LAYOUT ENGINE (deterministic fishbone; analytic collision pushing)
   ─────────────────────────────────────────────────────────────────────────── */
type Disc = { x:number; y:number; r:number; id?:string };
type SegV = { x:number; y1:number; y2:number };
type Arc  = { cx:number; cy:number; r:number };
type HRun = { x0:number; x1:number; y:number };

function layoutFishbone(g:Graph){
  computeDepths(g);
  const root:any = getRoot(g); root.x = 0; root.y = 0;

  // Update global max contributors for relative radius scaling
  state.maxContrib = Math.max(1, ...Object.values(g).map(n => n.contributors || 0));

  const discs: Disc[] = [{ x:root.x, y:root.y, r:rFor(root.contributors), id:root.id }];
  const trunks: SegV[] = []; const arcs: Arc[] = []; const runs: HRun[] = [];
  const parents = Object.values(g).sort((a:any,b:any)=> (a.depth - b.depth));

  for(const p of parents){
    const kids = p.children.map(id=>g[id]);
    if(!kids.length) continue;

    const px = p.x ?? 0, py = p.y ?? 0, pr = rFor(p.contributors);
    const baseY = (p.depth + 1) * LEVEL_GAP;
    const yStart = py + pr + STEM_LEN_PARENT;
    const R = state.elbowR;

    const leftLane: Array<[number,number]> = [];
    const rightLane: Array<[number,number]> = [];
    let turn: Side = -1;

    const ordered = (state.scenario==="reference") ? kids.slice()
                    : kids.slice().sort((a,b)=>rFor(b.contributors)-rFor(a.contributors));
    let prevJoint = yStart - state.branchSpacing;

    const reserveLane = (lane: Array<[number,number]>, y:number, r:number) => lane.push([y - r - state.lanePad, y + r + state.lanePad]);
    const pushPastLane = (lane: Array<[number,number]>, y:number, r:number) => {
      for (const [a,b] of lane) if (!(y + r + state.lanePad < a || y - r - state.lanePad > b)) y = b + state.lanePad + r;
      return y;
    };

    for(const c of ordered){
      const cr = rFor(c.contributors);

      let side: Side;
      if (state.scenario==="reference" && c.sideHint) side = c.sideHint;
      else {
        const firstFree = (lane: Array<[number,number]>) => {
          let y = baseY;
          for(const [a,b] of lane) if(!(y+cr+state.lanePad<a || y-cr-state.lanePad>b)) y=b+state.lanePad+cr;
          return y;
        };
        const yL = firstFree(leftLane), yR = firstFree(rightLane);
        side = (yL===yR) ? (turn=(turn===-1?+1:-1)) : (yL<yR?-1:+1);
      }

      const minOffset = Math.max(state.hOffset, state.pathPad + 1, R + state.pathPad + 1);
      const cx = px + side * (cr + minOffset);

      let reqY = Math.max(baseY, yStart + R, prevJoint + state.branchSpacing + R);
      reqY = pushPastLane(side===-1?leftLane:rightLane, reqY, cr);

      const bubblePad = state.bubblePad, pathPad = state.pathPad;

      for (const d of discs) {
        if (d.id === p.id) continue;
        const dx = cx - d.x, sum = cr + d.r + bubblePad, absx = Math.abs(dx);
        if (absx < sum) reqY = Math.max(reqY, d.y + Math.sqrt(sum*sum - absx*absx));
      }
      for (const a of arcs) {
        const dx = cx - a.cx, sum = cr + a.r + pathPad, absx = Math.abs(dx);
        if (absx < sum) reqY = Math.max(reqY, a.cy + Math.sqrt(sum*sum - absx*absx));
      }
      for (const r of runs) {
        const A = Math.min(r.x0, r.x1), B = Math.max(r.x0, r.x1);
        const xClamp = Math.max(A, Math.min(cx, B));
        const dx = cx - xClamp, need = cr + pathPad;
        if (Math.abs(dx) < need) reqY = Math.max(reqY, r.y + Math.sqrt(need*need - dx*dx));
      }
      for (const d of discs) {
        if (d.id === p.id) continue;
        const dx = px - d.x, sum = R + d.r + pathPad, absx = Math.abs(dx);
        if (absx < sum) reqY = Math.max(reqY, d.y + Math.sqrt(sum*sum - absx*absx));
      }
      { const run0 = px + side*R, run1 = cx - side*STEM_LEN_CHILD; const A = Math.min(run0, run1), B = Math.max(run0, run1);
        for (const d of discs) {
          const xC = Math.max(A, Math.min(d.x, B)); const dx = d.x - xC, need = d.r + pathPad;
          if (Math.abs(dx) < need) reqY = Math.max(reqY, d.y + Math.sqrt(need*need - dx*dx));
        } }
      for (const s of trunks) if (Math.abs(cx - s.x) < cr + pathPad && reqY <= s.y2) reqY = s.y2 + cr + pathPad;

      reqY = pushPastLane(side===-1?leftLane:rightLane, reqY, cr);
      if (state.scenario === "reference") reqY = Math.min(reqY, baseY + MAX_REF_DROP);

      (c as any).x = cx; (c as any).y = reqY;
      reserveLane(side===-1?leftLane:rightLane, reqY, cr);
      discs.push({ x: cx, y: reqY, r: cr, id: c.id });
      arcs.push({ cx: px, cy: reqY, r: R });
      runs.push({ x0: px + side*R, x1: cx - side*STEM_LEN_CHILD, y: reqY });
      prevJoint = reqY - R;
    }

    const childYs = p.children.map(id => (g as any)[id].y ?? baseY);
    const lastJoint = (childYs.length ? Math.max(...childYs) - state.elbowR : yStart);
    trunks.push({ x: px, y1: py + pr, y2: lastJoint });

    if (!discs.find(d => d.id === p.id)) discs.push({ x: px, y: py, r: pr, id: p.id });
  }

  // Prepare arrays for Vue rendering
  nodesList.value = Object.values(g) as any;

  const links = nodesList.value.filter(n=>n.parentId).map(n => ({ source: (g as any)[n.parentId!], target: n }));
  const R = state.elbowR;
  const edges = links.map(l=>{
    const side: Side = (l.target.x! >= l.source.x!) ? +1 : -1;
    const ex = l.source.x!, ey = l.target.y! - R, hx = ex + side*R, hy = l.target.y!;
    const rt = rFor(l.target.contributors), cx = l.target.x! - side*(rt + STEM_LEN_CHILD), cy = hy;
    const sx1 = l.target.x! - side*rt, sy1 = hy, sx2 = cx, sy2 = cy;
    return { source:l.source, target:l.target, side, ex, ey, hx, hy, cx, cy, sx1, sy1, sx2, sy2 };
  });
  edgesList.value = edges;

  trunksList.value = nodesList.value.filter(n=>n.children.length>0).map(n=>{
    const rs = rFor(n.contributors);
    const yStart = n.y! + rs + STEM_LEN_PARENT;
    const ys = n.children.map(id => (g as any)[id].y! - R);
    const y2 = Math.max(yStart, ...ys);
    return { x: n.x!, y1: n.y! + rs, y2, id: n.id };
  });

  jointDots.value = edges.map(e => ({ x:e.ex, y:e.ey, id:`${e.source.id}-${e.target.id}` }));

  const maxY = Math.max(...nodesList.value.map(n => (n.y ?? 0) + rFor(n.contributors)));
  svgHeight.value = Math.max(containerHeight, maxY + SVG_BOTTOM_PADDING);
}

/* ─────────────────────────────────────────────────────────────────────────────-
   VIEW FITTING (responsive reset + tiny-graph elegance)
   ─────────────────────────────────────────────────────────────────────────── */
function contentBounds(){
  const minX = Math.min(...nodesList.value.map(n => (n.x ?? 0) - rFor(n.contributors)));
  const maxX = Math.max(...nodesList.value.map(n => (n.x ?? 0) + rFor(n.contributors)));
  const minY = Math.min(...nodesList.value.map(n => (n.y ?? 0) - rFor(n.contributors)));
  const maxY = Math.max(...nodesList.value.map(n => (n.y ?? 0) + rFor(n.contributors)));
  // Account for elbow overhang beyond rightmost/leftmost bubbles
  const extraX = state.hOffset + state.elbowR + STEM_LEN_CHILD + CONTENT_BOUNDS_EXTRA;
  return { minX: minX - extraX, maxX: maxX + extraX, minY, maxY };
}

function resetView(animated=false){
  /* Centering fix: apply transform to worldSel (the same <g> Vue renders). */
  const svg = svgRef.value!;
  const box = svg.getBoundingClientRect();
  if (!box.width || !box.height) {
    requestAnimationFrame(() => resetView(animated));
    return;
  }
  const usableH = box.height - VIEW_TOP_OFFSET;

  const forks = forkCount(state.graph);
  const b = contentBounds();
  const contentW = b.maxX - b.minX, contentH = b.maxY - b.minY;

  // Calculate fill fraction based on number of forks (more forks = fill more of viewport)
  const fillFrac = FILL_FRACTION_MIN + (FILL_FRACTION_MAX - FILL_FRACTION_MIN) * Math.min(1, forks / COMPLEXITY_THRESHOLD);

  // Calculate scale to fit content horizontally and vertically
  const scaleW = (box.width * fillFrac) / Math.max(1, contentW);
  const scaleH = (usableH * VERTICAL_FILL_FRACTION) / Math.max(1, contentH);

  let targetScale = Math.min(scaleW, scaleH);
  // Special handling for single-fork graphs: make the bubble nicely sized
  if (forks <= 1) {
    const root = getRoot(state.graph);
    const r = rFor(root.contributors);
    const desiredD = Math.max(SINGLE_FORK_DIAMETER_MIN, Math.min(Math.floor(box.width * SINGLE_FORK_WIDTH_RATIO), SINGLE_FORK_DIAMETER_MAX));
    const sBubble  = desiredD / (2 * r);
    targetScale = Math.min(ZOOM_MAX, Math.max(sBubble, targetScale));
  }

  // Center horizontally
  const cx = box.width / 2;
  const worldCenterX = (b.minX + b.maxX) / 2;
  const tx = cx - (worldCenterX * targetScale);
  // Position vertically with top margin
  const targetTop = VIEW_TOP_OFFSET + RESET_TOP_MARGIN;
  const ty = targetTop - (b.minY * targetScale);

  const t = zoomIdentity.translate(tx, ty).scale(targetScale);
  (animated ? svgSel.transition().duration(VIEW_TRANSITION_DURATION) : svgSel).call(zoomBehavior.transform as any, t);

  currentK.value = targetScale;
}

/* Click focus: center selected bubble and fit fully */
function focusNode(n:Node){
  /* Note: also applied to worldSel via zoomBehavior, so it now works. */
  const svg = svgRef.value!;
  const box = svg.getBoundingClientRect();
  const usableH = box.height - VIEW_TOP_OFFSET;
  const r = rFor(n.contributors);
  
  // Calculate scale to fit the bubble with padding
  const sx = (box.width - 2 * FOCUS_PADDING) / (2 * r);
  const sy = (usableH - 2 * FOCUS_PADDING) / (2 * r);
  const scale = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, Math.min(sx, sy)));

  // Center the node in the viewport
  const cx = box.width / 2;
  const cy = VIEW_TOP_OFFSET + usableH / 2;
  const tx = cx - (n.x! * scale);
  const ty = cy - (n.y! * scale);
  const t = zoomIdentity.translate(tx, ty).scale(scale);
  svgSel.transition().duration(VIEW_TRANSITION_DURATION).call(zoomBehavior.transform as any, t);
  currentK.value = scale;
}

/* ─────────────────────────────────────────────────────────────────────────────-
   DATA MUTATION (demo add/remove)
   ─────────────────────────────────────────────────────────────────────────── */
function addFork(parentId:NodeId){
  const p = state.graph[parentId];
  if (!p) return;
  
  // Check depth to prevent explosion
  const depth = (function getDepth(id:NodeId){
    let d = 0, cur = state.graph[id];
    while (cur.parentId) {
      d++;
      cur = state.graph[cur.parentId];
    }
    return d;
  })(parentId);
  if (depth >= MAX_DEPTH - 1) return;

  // Generate random contributor count and ID for demo fork
  const n = RANDOM_MIN + Math.floor(Math.random() * (RANDOM_MAX - RANDOM_MIN + 1));
  const id = Math.random().toString(36).slice(RANDOM_ID_SLICE_START, RANDOM_ID_SLICE_START + RANDOM_ID_LENGTH);
  state.graph[id] = {
    id,
    contributors: n,
    parentId,
    children: [],
    updatedAt: new Date().toISOString().slice(0, 10)
  };
  p.children.push(id);

  layoutAndRender();
  resetView(true);
}

function deleteNode(id:NodeId){
  const node=state.graph[id]; if(!node||node.parentId===null) return;
  const parent=state.graph[node.parentId];
  for(const cid of node.children){ const c=state.graph[cid]; if(!c) continue; c.parentId=parent.id; parent.children.push(cid); }
  parent.children=parent.children.filter(x=>x!==id);
  delete state.graph[id];
  layoutAndRender(); resetView(true);
}

/* ─────────────────────────────────────────────────────────────────────────────-
   RENDER PIPELINE (layout→derive arrays→Vue renders)
   ─────────────────────────────────────────────────────────────────────────── */
function layoutAndRender(){
  applyResponsiveDials();          // adapt dials first
  layoutFishbone(state.graph);     // compute x,y and derive edges/trunks/lists
}

/* ─────────────────────────────────────────────────────────────────────────────-
   MOUNT (zoom wiring, resize observer, seeds)
   ─────────────────────────────────────────────────────────────────────────── */
onMounted(async ()=>{
  svgSel   = select(svgRef.value!);
  worldSel = select(worldRef.value!);  // CRITICAL: the very group Vue renders into

  zoomBehavior = zoom()
    .scaleExtent([ZOOM_MIN, ZOOM_MAX])
    /* Filter: pinch and ctrl+wheel zoom; plain wheel should pan (handled below). */
    .filter((event:any) => event.type === "wheel" ? event.ctrlKey : true)
    .on("zoom",(e:any)=>{
      const z:ZoomTransform = e.transform; currentK.value = z.k;
      /* Apply pan/zoom to the SAME world group that holds all nodes/edges. */
      worldSel.attr("transform", z.toString());
    });

  svgSel.call(zoomBehavior as any);

  /* Background click (outside any bubble) → reset; if on true background (svg), also clear selection */
  svgSel.on("click.bg", (ev:any)=>{
    const target = ev.target as Element;
    if (!target.closest("g.node")) {
      resetView(true);
      applySelection(null, null);
      pendingExternalSelection = null;
      persistSelectionDetail(null);
      window.dispatchEvent(new CustomEvent('repo:bubble-selected', {detail: null}));
      window.dispatchEvent(new CustomEvent('repo:selection-updated', {detail: null}));
    }
  });

  /* Wheel pans (natural trackpad behavior). Ctrl+wheel handled by d3-zoom. */
  svgSel.on("wheel.pan",(ev:any)=>{
    if (ev.ctrlKey) return;       // let ctrl+wheel zoom handler run
    ev.preventDefault();
    svgSel.call(zoomBehavior.translateBy as any, -ev.deltaX, -ev.deltaY);
  }, { passive:false });

  /* Observe container width for responsive dials */
  await nextTick();
  const el = containerRef.value;
  if (!el) {
    console.warn('FishboneGraph: container element not available');
    return;
  }
  const rect0 = el.getBoundingClientRect();
  containerWidth = rect0.width;
  containerHeight = rect0.height;
  ro = new ResizeObserver((entries)=>{
    const rect = entries[0].contentRect;
    const w = rect.width;
    const h = rect.height;
    let changed = false;
    if (Math.abs(w - containerWidth) > 2) { containerWidth = Math.min(w, 1100); changed = true; }
    if (Math.abs(h - containerHeight) > 2) { containerHeight = h; changed = true; }
    if (changed) {
      if (pendingRaf !== null) cancelAnimationFrame(pendingRaf);
      pendingRaf = requestAnimationFrame(()=>{
        layoutAndRender();
        resetView();
        pendingRaf = null;
      });
    }
  });
  ro.observe(el);

  /* Initial fetch from API */
  await fetchForkGraphAndSet();
  window.addEventListener('repo:selection-updated', handleExternalSelection as EventListener);
});

onBeforeUnmount(()=>{
  if (ro) ro.disconnect();
  window.removeEventListener('repo:selection-updated', handleExternalSelection as EventListener);
});

/* Derived for template binding */
const kComputed = computed(()=> currentK.value);

function persistSelectionDetail(detail: RepoSelectionDetail | null) {
  if (typeof window === 'undefined') return;
  try {
    if (!detail) {
      window.localStorage.removeItem(LS_OWNER_KEY);
      window.localStorage.removeItem(LS_SUBJECT_KEY);
      window.localStorage.removeItem(LS_REPO_KEY);
    } else {
      window.localStorage.setItem(LS_OWNER_KEY, detail.owner);
      if (detail.subject) {
        window.localStorage.setItem(LS_SUBJECT_KEY, detail.subject);
      } else {
        window.localStorage.removeItem(LS_SUBJECT_KEY);
      }
      window.localStorage.setItem(LS_REPO_KEY, detail.repo);
    }
  } catch {
    // ignore storage quotas
  }
}

/* Click handler: focus or delete, and persist selected article (owner/subject) */
function onBubbleClick(n: Node, ev: MouseEvent){
  if (ev && (ev as any).altKey) { 
    deleteNode(n.id); 
    announceToScreenReader(`Removed ${n.fullName || n.id}`);
    return; 
  }
  focusNode(n);
  const detail = getSelectionDetailFromNode(n);
  if (!detail) return;
  const payload = {...detail};
  applySelection(n, payload);
  persistSelectionDetail(payload);
  announceToScreenReader(`Selected ${n.fullName || n.id} with ${n.contributors} contributor${n.contributors === 1 ? '' : 's'}`);
  window.dispatchEvent(new CustomEvent('repo:bubble-selected', {detail: payload}));
  window.dispatchEvent(new CustomEvent('repo:selection-updated', {detail: payload}));
}

function onBubbleView(n: Node){
  const detail = getSelectionDetailFromNode(n);
  if (!detail) return;
  const payload = {...detail};
  applySelection(n, payload);
  persistSelectionDetail(payload);
  window.dispatchEvent(new CustomEvent('repo:selection-updated', {detail: payload}));
  window.dispatchEvent(new CustomEvent('repo:bubble-open-article', {detail: payload}));
}
</script>

<template>
    <div class="f-fishbone-graph" ref="containerRef">
      <!-- Screen reader announcements -->
      <div class="sr-only" role="status" aria-live="polite" aria-atomic="true">{{ srAnnouncement }}</div>
      <div class="mx-auto max-w-[1100px] relative">
      <!-- Controls removed; using defaults -->

      <!-- SVG world: IMPORTANT → touch-action:none enables pinch zoom; d3 handles it -->
      <svg ref="svgRef" 
           class="tw-w-full" 
           :style="{ height: svgHeight + 'px' }" 
           style="touch-action: none;"
           role="img"
           aria-label="Fork repository graph showing contributors and relationships"
           tabindex="0">
        <defs>
          <!-- Soft radial bubble gradient -->
          <radialGradient id="bubbleGrad" cx="35%" cy="30%" r="65%">
            <stop offset="0%"  stop-color="#FAFBFC"/>
            <stop offset="60%" stop-color="#EEF2F7"/>
            <stop offset="100%" stop-color="#E6EBF2"/>
          </radialGradient>
          <filter id="softShadow" x="-50%" y="-50%" width="200%" height="200%">
            <feDropShadow dx="0" dy="2" stdDeviation="3" flood-color="#64748b" flood-opacity="0.18"/>
          </filter>
        </defs>

        <!-- WORLD GROUP: Vue renders here, and d3-zoom transforms this exact <g> -->
        <g ref="worldRef">
          <!-- Trunks (vertical) -->
          <line v-for="t in trunksList" :key="t.id"
                class="trunk"
                :x1="t.x" :x2="t.x"
                :y1="t.y1" :y2="t.y2"
                stroke="#D7DFE8" stroke-width="2" stroke-linecap="round" />

          <!-- Branch elbows + runs (one path per edge) -->
          <path v-for="e in edgesList" :key="`${e.source.id}-${e.target.id}`"
                class="branch" fill="none" stroke="#D7DFE8" stroke-width="2" stroke-linecap="round" opacity="0.9"
                :d="`M ${e.ex} ${e.ey} C ${e.ex} ${e.ey + 0.5522847498307936*state.elbowR}, ${e.ex + e.side*0.5522847498307936*state.elbowR} ${e.hy}, ${e.hx} ${e.hy} L ${e.cx} ${e.cy}`" />

          <!-- Child stems -->
          <line v-for="e in edgesList" :key="`stem-${e.source.id}-${e.target.id}`"
                class="child-stem"
                :x1="e.sx1" :y1="e.sy1" :x2="e.sx2" :y2="e.sy2"
                stroke="#D7DFE8" stroke-width="2" stroke-linecap="round" opacity="0.9" />

          <!-- Joint dots (hollow rings) on trunk side -->
          <circle v-for="j in jointDots" :key="`joint-${j.id}`"
                  class="joint-parent" :cx="j.x" :cy="j.y" r="6"
                  fill="#ffffff" stroke="#C7D2DF" stroke-width="2" />
          
          <!-- Bubbles (component handles labels independently) -->
          <BubbleNode v-for="n in nodesList" :key="n.id"
            :id="n.id" :x="(n as any).x" :y="(n as any).y" :r="(rFor(n.contributors))"
            :contributors="n.contributors" :updatedAt="n.updatedAt" :k="kComputed"
            :is-active="selectedNodeId === n.id"
            @click="(_, ev) => onBubbleClick(n, ev)"
            @view="() => onBubbleView(n)"
            @dblclick="() => addFork(n.id)" />
        </g>
      </svg>

      <LegendFishbone />
    </div>
  </div>
</template>

<style scoped>
.f-fishbone-graph {
  width: 100%;
  height: calc(100vh - 25rem);
  overflow: auto;
}

.f-fishbone-graph svg:focus {
  outline: none;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border-width: 0;
}
</style>
