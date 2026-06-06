# Polyglot Claude Code Usability Test — Protocol

5-user Krug-style observed usability test. Validates the agent-mode
onboarding fixes shipped in the wedge re-anchoring iteration before we
scale community-led GTM.

This document is the operating protocol: who to recruit, what to ask,
how to run the session, how to score. Findings template lives at
`docs/research/usability-test-findings-template.md` — a copy of that
file becomes the per-cohort report.

## Goal

Decide whether three specific onboarding fixes work for the primary ICP
(Taylor, the AI-coding polyglot developer):

1. **Agent-mode discovery** — does a Claude Code/Cursor user encounter
   the MCP integration unprompted, or do they default to the terminal
   path?
2. **Failure-mode recovery** — when the first `coverctl check` fails,
   does the user know what to do next (failure-mode caution blocks +
   inline CLI footer)?
3. **Time-to-first-fix** — install → first regression caught and
   addressed in under 10 minutes? On what kinds of repos does that
   target hold?

Out of scope: feature requests, comparative evaluation against
Codecov, deep accessibility audit. Stay narrow.

## Cohort

5 participants, each currently using Claude Code or Cursor at least
weekly:

| # | Language stack | Why |
| --- | --- | --- |
| 1 | Python + TypeScript | Most common ECP stack |
| 2 | Python + TypeScript | Confirms first observation |
| 3 | Go + Rust | Strong-typing cohort, weight on Go retention |
| 4 | Go + Rust | Confirms third observation |
| 5 | Java or Shell | Smaller-ecosystem cohort, surfaces weak-runner gaps |

Hard requirements:
- Active Claude Code or Cursor user (last 30 days, weekly+ usage)
- Currently working on a polyglot codebase (≥2 languages)
- Has at least one CI policy gate today (any kind)

Soft preferences:
- Mix of company sizes (solo, 5-50, 50-500)
- Mix of domain types (web app, infra, data, agentic system)
- Avoid early coverctl users — we want fresh first-run reactions

## Recruitment

Sources, in order of preference:

1. Claude Code Discord `#show-and-tell` and `#feedback` channels —
   post a brief ask, no incentive needed for the first session
2. r/ClaudeAI and r/cursor subreddits — Sunday post, tag as
   "research"
3. Personal network (1-2 max — bias risk)
4. Hacker News "Who is hiring?" replies — last resort, slow

Post template lives in `docs/research/recruitment-post.md` — see below.

Incentive: $50 gift card per session. Skip incentive if recruiting
through personal network.

Time commitment to advertise: 45 min. Buffer 60 min in your calendar.

### Recruitment post (use as-is)

> **Subject:** 45-min user research session ($50) — coverctl + Claude Code
>
> I am running a small usability test for **coverctl**, a CLI + MCP
> server that gives AI coding agents in-loop coverage feedback before
> commit. I'd like to watch you install it from scratch and use it
> through Claude Code or Cursor on your own polyglot codebase.
>
> If you fit:
>
> - Use Claude Code or Cursor weekly
> - Working in a polyglot repo (≥2 languages)
> - Have any CI policy gate today
>
> 45 min over Zoom or similar. Screen-share required. $50 gift card
> after. DM me a 2-line note about your stack.

## Screener (5 questions, 5 minutes)

Run this before booking. Disqualify if any answer is no.

1. Which AI coding tool do you use most weeks? (Need: Claude Code,
   Cursor, or Cline)
2. How many distinct languages does your main codebase use? (Need: ≥2)
3. Do you have any CI gate today — lint, test, type, coverage —
   that blocks merge if it fails? (Need: yes)
4. In the last sprint, has CI failed on a coverage regression in a PR
   you were involved with? (Want: yes — it primes the wedge)
5. Have you used coverctl or coverctl plugins before? (Need: no)

## Session structure (45 min)

Cap each phase at the listed time. Cut rather than overrun.

| Phase | Time | What |
| --- | --- | --- |
| Intro + consent | 5 min | Explain purpose, get screen-share recording consent |
| Background interview | 5 min | Stack, AI usage, current coverage tooling, last red-CI moment |
| Discovery task | 10 min | "You want to add coverage governance to this repo. Find a tool, install it, and reach a first useful signal. Think aloud." (Do **not** mention coverctl by name yet.) |
| Coverctl install | 10 min | If they did not pick coverctl spontaneously, hand them the URL: `https://klarlabs-studio.github.io/coverctl/`. Continue think-aloud. |
| First failure recovery | 10 min | Whatever they hit first (init failure, FAIL on first check, missing toolchain). Watch how they react. Do not help unless they hit a 3-min stall. |
| Debrief | 5 min | Three closing questions (below) |

### Closing questions

Ask in this order, do not skip:

1. "On a scale of 1-5, how likely are you to use coverctl on this repo
   tomorrow morning? Why?"
2. "What was the most confusing moment? What did you expect to happen?"
3. "If you could only keep one thing about coverctl, what would it be?
   Cut everything else."

## Observation rubric

For each participant record verbatim quotes plus rubric scores during
the session. Do not retro-fill from memory.

| Signal | Score | What to look for |
| --- | --- | --- |
| **Agent-mode discovery** | did-they-find / had-to-be-told | Did the user navigate to `quick-start-agent` unprompted from the landing page? |
| **First-fix time** | minutes-to-first-fix | Stopwatch from `brew install` (or equivalent) to first reaction-of-relief moment |
| **Failure recovery (init)** | self-recovered / asked-for-help | When `init` failed (mixed repo, permissions, no marker), did the caution block resolve them? |
| **Failure recovery (check)** | self-recovered / asked-for-help | When the first `check` hit FAIL, did the inline footer + caution block point them at a clear next action? |
| **MCP setup** | success-without-help / needed-doc / abandoned | Were they able to wire coverctl into their agent client? |
| **Trust calibration** | calibrated / over-trusted / under-trusted | Did they treat coverctl output as authoritative, hallucinate confidence, or distrust it incorrectly? |
| **First emotional peak** | what-and-when | Note exact moment of first positive reaction; this is the Peak-End anchor |
| **First friction peak** | what-and-when | Note the worst moment; this is the design hot spot |

## Consent + recording

Use this consent text verbatim. Pause and read it; do not paraphrase.

> "I'd like to record screen and voice for this session. The recording
> stays with me, is not published, and is deleted within 30 days.
> Findings I write will reference your role and stack, never your name
> or company. You can stop the session and have the recording deleted
> at any time. Are you okay with that?"

If the participant declines recording, run the session anyway and
take notes only. Do not pressure.

## Anti-pattern checklist (interviewer)

Read before each session.

- **Do not** demo coverctl. Watch them discover and use it.
- **Do not** explain what a tool *should* do. If asked, deflect: "what
  do you think it should do?"
- **Do not** ask about features. Ask about struggling moments
  (Moesta).
- **Do not** lead — "would you find X useful?" is biased. Use:
  "what would you have hoped for at this moment?"
- **Do not** rescue too early. Three minutes of stuck silence is
  research data, not a problem to fix mid-session.
- **Do not** end on a positive note artificially. End-of-session
  emotion is part of Peak-End — note it, don't manufacture it.

## After each session

Within 30 minutes of session end:

1. Save recording (encrypted folder; 30-day TTL).
2. Fill out one row in `docs/research/usability-test-findings-template.md`.
3. Write one verbatim quote per session — the strongest, even if
   uncomfortable.
4. Tag the dominant friction point with one of: discovery, install,
   init, check, MCP-setup, agent-trust, fix-loop.

After all 5 sessions: aggregate per the findings template; share with
yourself + the gtm-expert / ux-expert lenses; decide what ships in the
next iteration.

## Stop conditions

Stop the cohort early — do not run all 5 — if any of these fire:

- 2 consecutive participants cannot reach first fix in <15 min
  (target was <10). Iteration on docs/CLI before continuing.
- 3 consecutive participants ignore agent-mode entirely. Landing-page
  hero needs rework before continuing.
- A participant hits a security or correctness bug. Fix first, resume.

These are early-exit signals; running through all 5 wastes attention
when the design is already clearly broken.
