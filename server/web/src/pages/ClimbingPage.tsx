import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import {
  fetchClimbingSessions,
  type ClimbingAscent,
  type ClimbingSession,
} from "../api";
import PageHeader from "../components/PageHeader";
import RangeControl from "../components/RangeControl";
import { useIsDesktop } from "../hooks/useMediaQuery";
import { formatClock, formatDateWithYear, formatShortDate } from "../utils/format";
import { queryMessage, queryState } from "../utils/queryState";

const RANGES = ["30d", "90d", "1y"] as const;
type Range = (typeof RANGES)[number];
const RANGE_DAYS: Record<Range, number> = { "30d": 30, "90d": 90, "1y": 365 };

const RESULT_LABEL: Record<ClimbingAscent["result"], string> = {
  flash: "flashed",
  top: "topped",
  repeat: "repeated",
  attempt: "tried",
};

/** Routes topped in the session, first try or not; repeats count separately. */
function isTop(a: ClimbingAscent) {
  return a.result === "flash" || a.result === "top";
}

function counts(ascents: ClimbingAscent[]) {
  const c = { flash: 0, top: 0, repeat: 0, attempt: 0 };
  for (const a of ascents) c[a.result] += 1;
  return c;
}

/**
 * The grades of the routes topped, hardest first, with a count per grade.
 * Ordered by grade_rank, which only compares grades within one gym's scale —
 * so this is computed per session, never across sessions.
 */
function gradeLine(ascents: ClimbingAscent[]) {
  const byGrade = new Map<string, { rank: number; n: number }>();
  for (const a of ascents.filter(isTop)) {
    const key = a.grade || "ungraded";
    const g = byGrade.get(key) ?? { rank: a.grade_rank ?? -1, n: 0 };
    g.n += 1;
    byGrade.set(key, g);
  }
  return [...byGrade.entries()]
    .sort((a, b) => b[1].rank - a[1].rank)
    .map(([grade, g]) => (g.n > 1 ? `${grade} ×${g.n}` : grade))
    .join(" · ");
}

export default function ClimbingPage() {
  const isDesktop = useIsDesktop();
  const [params, setParams] = useSearchParams();
  const range = (params.get("range") as Range) ?? "90d";

  const end = new Date();
  const start = new Date(end.getTime() - RANGE_DAYS[range] * 86400000);
  // The end bound is exclusive, so tomorrow's date keeps today's sessions in.
  const endISO = new Date(end.getTime() + 86400000).toISOString().split("T")[0];
  const startISO = start.toISOString().split("T")[0];

  const query = useQuery({
    queryKey: ["climbing-sessions", startISO, endISO],
    queryFn: () => fetchClimbingSessions(startISO, endISO),
  });
  const message = queryMessage(queryState(query), query.error);
  // Logbook apps also record visits with nothing logged; they carry no data.
  const sessions = useMemo(
    () => (query.data ?? []).filter((s) => s.ascents.length > 0),
    [query.data],
  );

  const total = useMemo(
    () => counts(sessions.flatMap((s) => s.ascents)),
    [sessions],
  );

  return (
    <>
      <PageHeader
        kicker={`${formatDateWithYear(start)} – ${formatDateWithYear(end)}`}
        title="Climbing"
        actions={
          <RangeControl
            options={RANGES}
            value={range}
            onChange={(v) => {
              const p = new URLSearchParams(params);
              p.set("range", v);
              setParams(p, { replace: true });
            }}
            name="climbing-range"
          />
        }
      />

      <div
        style={{
          display: "grid",
          gridTemplateColumns: isDesktop ? "repeat(4, 1fr)" : "repeat(2, 1fr)",
          borderTop: "2px solid var(--color-text)",
          borderBottom: "2px solid var(--color-text)",
        }}
      >
        <Cell label="Sessions" value={sessions.length} isDesktop={isDesktop} />
        <Cell label="Tops" value={total.flash + total.top} isDesktop={isDesktop} />
        <Cell label="Flashes" value={total.flash} isDesktop={isDesktop} />
        <Cell label="Tried" value={total.attempt} isDesktop={isDesktop} />
      </div>

      {message ? (
        <p className="page-x" style={{ color: "var(--color-neutral-600)", fontSize: 13 }}>
          {message}
        </p>
      ) : sessions.length === 0 ? (
        <p className="page-x" style={{ color: "var(--color-neutral-600)", fontSize: 13 }}>
          No climbing sessions in this window.
        </p>
      ) : (
        <div>
          {sessions.map((s) => (
            <SessionRow key={`${s.source}/${s.id}`} session={s} />
          ))}
        </div>
      )}
    </>
  );
}

function Cell({
  label,
  value,
  isDesktop,
}: {
  label: string;
  value: number;
  isDesktop: boolean;
}) {
  return (
    <div
      className="page-x"
      style={{
        paddingTop: isDesktop ? 24 : 14,
        paddingBottom: isDesktop ? 22 : 14,
        borderRight: "1px solid var(--color-divider)",
      }}
    >
      <div className="kick">{label}</div>
      <div
        className="num"
        style={{
          font: `800 ${isDesktop ? 44 : 26}px/1 var(--font-heading)`,
          letterSpacing: "-0.03em",
          marginTop: isDesktop ? 14 : 8,
        }}
      >
        {value}
      </div>
    </div>
  );
}

function SessionRow({ session }: { session: ClimbingSession }) {
  const c = counts(session.ascents);
  const summary = (Object.keys(RESULT_LABEL) as ClimbingAscent["result"][])
    .filter((r) => c[r] > 0)
    .map((r) => `${c[r]} ${RESULT_LABEL[r]}`)
    .join(" · ");
  const grades = gradeLine(session.ascents);

  return (
    <div
      className="page-x"
      style={{
        paddingTop: 12,
        paddingBottom: 12,
        borderBottom: "1px solid var(--color-divider)",
      }}
    >
      <div style={{ display: "flex", alignItems: "baseline", gap: 10 }}>
        <span style={{ font: "600 14px var(--font-body)", flex: 1 }}>
          {formatShortDate(new Date(session.start))} · {session.gym || session.source}
        </span>
        <span className="num" style={{ font: "500 11.5px var(--font-body)", color: "var(--color-neutral-600)" }}>
          {formatClock(session.start)}
        </span>
      </div>
      <div style={{ font: "400 12.5px var(--font-body)", marginTop: 4 }}>{summary}</div>
      {grades ? (
        <div
          style={{
            font: "400 11.5px var(--font-body)",
            color: "var(--color-neutral-600)",
            marginTop: 2,
          }}
        >
          {grades}
        </div>
      ) : null}
    </div>
  );
}
