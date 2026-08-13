import { Play } from 'lucide-react'

import type { CallResult, RoutineArg, SourceKind } from '@/bridge/schema'
import type { Cell } from '@/bridge/tables'
import { ResultGrid } from '@/features/results/ResultGrid'
import type { GridData } from '@/features/results/resultSet'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { Loading } from '@/shared/ui/Loading'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { SOURCE_NOUN } from '@/stores/tabs'

import type { RoutineCall } from './useRoutineCall'

interface RoutineRunnerProps {
  kind: SourceKind
  call: RoutineCall
}

/** The one editable column of the arguments grid. Everything left of it is the catalogue's. */
const VALUE = 3

/**
 * Running a routine with its arguments filled in. It is the third pane of the definition tab,
 * beside the statement that declares one and the statements a Save would run, because that is where
 * a reader who has just edited a procedure is standing when they want to know whether it works.
 *
 * It draws what `useRoutineCall` holds and nothing else — the state is the *tab's*, so a look at
 * the definition and back does not clear the boxes.
 *
 * A value travels as a **string literal** whatever the parameter's type is, which every one of these
 * engines reads back into a parameter of any type. An empty box is `NULL` — the keyword and not the
 * word — and an OUT parameter has no value to type at all, being what the call answers with.
 */
export function RoutineRunner({ kind, call }: RoutineRunnerProps) {
  const { args, statements, answer, running, error } = call

  if (args === null) return <Loading label="Reading what it takes…" />

  const noun = SOURCE_NOUN[kind]
  const lower = noun.toLowerCase()

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex h-9 shrink-0 items-center gap-1.5 border-b border-line bg-bar px-2">
        {/* Dead where the engine spelled nothing: a call it has no grammar for is not a button that
            fails, it is a button there is nothing behind. */}
        <Button
          variant="primary"
          icon={<Play size={13} />}
          disabled={running || statements.length === 0}
          onClick={() => void call.run()}
        >
          {running ? 'Running…' : `Run ${noun}`}
        </Button>

        <span className="truncate text-2xs text-faint">
          {args.length === 0
            ? `This ${lower} is called with nothing.`
            : 'An empty box is NULL. Every value is written in as a literal.'}
        </span>

        <div className="flex-1" />
        {answer ? (
          <span className="shrink-0 rounded-md border border-line bg-canvas px-1.5 py-0.5 font-mono text-2xs text-faint">
            {said(answer)}
          </span>
        ) : null}
      </div>

      {error ? (
        <div className="shrink-0 px-2 pt-2">
          <Banner tone="danger" text={error.message} />
        </div>
      ) : null}

      <div className="min-h-0 flex-1">
        <Split
          id="routine-run"
          orientation="vertical"
          panelIds={['arguments', 'answer']}
          className="h-full"
        >
          <SplitPanel id="arguments" defaultSize={45} minSize={20}>
            <div className="flex h-full min-h-0 flex-col">
              <div className="min-h-0 flex-1">
                {args.length === 0 ? (
                  <Empty
                    title="No arguments"
                    hint={`This ${lower} takes nothing. Run it to see what it answers.`}
                  />
                ) : (
                  <ResultGrid
                    result={argumentRows(args)}
                    onEditCell={({ row, column }, value) => {
                      if (column === VALUE) call.setArg(row, value ?? '')
                    }}
                  />
                )}
              </div>

              {/* What Run will send, in order — `PreviewCall`, which is the very list `CallRoutine`
                  runs. It is under the boxes rather than in a pane of its own because it is the one
                  thing a reader has to have seen before pressing the button above. */}
              <div className="max-h-24 shrink-0 overflow-auto border-t border-line bg-panel px-2 py-1">
                {statements.length === 0 ? (
                  <p className="font-mono text-2xs text-faint">Nothing to run.</p>
                ) : (
                  statements.map((statement, at) => (
                    <pre
                      key={at}
                      className="selectable font-mono text-2xs whitespace-pre-wrap text-muted"
                    >
                      {statement}
                    </pre>
                  ))
                )}
              </div>
            </div>
          </SplitPanel>

          <SplitHandle orientation="vertical" />

          <SplitPanel id="answer" defaultSize={55} minSize={20}>
            {answer === null ? (
              <Empty
                title="Not run yet"
                hint={`Run ${noun} sends the statements under the arguments, in that order.`}
              />
            ) : answer.columns.length === 0 ? (
              <Empty title={said(answer)} hint={`This ${lower} answered with no rows.`} />
            ) : (
              <ResultGrid result={{ columns: answer.columns, rows: answer.rows }} />
            )}
          </SplitPanel>
        </Split>
      </div>
    </div>
  )
}

/**
 * The arguments as rows and columns, which is the one component this app has for either. The three
 * columns in front of the value are the **catalogue's** and are read-only: what a routine takes is
 * the routine's, and a reader who wants it different edits the definition in the pane beside this.
 *
 * An OUT parameter's cell says so rather than being blank — a blank one reads as a value nobody has
 * typed yet, when it is a value the reader will be *given*.
 */
function argumentRows(args: RoutineArg[]): GridData {
  return {
    columns: [
      { name: 'Name', type: 'text', fullType: 'text', readOnly: true },
      { name: 'Mode', type: 'text', fullType: 'text', readOnly: true },
      { name: 'Type', type: 'text', fullType: 'text', readOnly: true },
      { name: 'Value', type: 'text', fullType: 'text' },
    ],
    rows: args.map((arg): Cell[] => [
      arg.name,
      arg.mode,
      arg.type,
      arg.mode === 'out' ? '(given back)' : (arg.value ?? ''),
    ]),
  }
}

/**
 * What the run came to: its rows where it answered with any, and what it changed where it did not.
 * A routine that answered with neither says how long it took and nothing else — `0 affected` would
 * be a claim about a program whose writes this side never counted.
 */
function said(answer: CallResult): string {
  if (answer.columns.length > 0) return `${answer.rows.length} rows · ${answer.elapsedMs} ms`
  if (answer.rowsAffected > 0) return `${answer.rowsAffected} affected · ${answer.elapsedMs} ms`
  return `Ran in ${answer.elapsedMs} ms`
}
