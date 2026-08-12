import { completeFromList, type CompletionSource } from '@codemirror/autocomplete'
import { StreamLanguage } from '@codemirror/language'
import type { Extension } from '@codemirror/state'

import type { Snippet } from '@/bridge/snippets'

import { saved } from './snippets'

/**
 * The editor for a key-value store. A Redis command is **argv** — a verb and then bytes — so there
 * is no grammar to parse and nothing to indent: what colouring can honestly say is which word is the
 * verb, which run of characters is quoted, and where a comment starts.
 *
 * That is all this does. **Where a command ends and what it is going to do are the driver's**
 * (`internal/driver/redis/text.go`, over `sqlkit.Lex`), and the editor asks for them rather than
 * working them out again — see the rule in CLAUDE.md and docs/editor.md. Nothing here decides
 * anything; it paints, and it offers a list of names.
 */

interface CommandState {
  /** How many words into the command the stream is, which is the only thing colouring needs. */
  at: number
  /** The verb, so a container command's second word can be coloured as part of it. */
  head: string
}

/** Every command whose verb is two words, which is why the second one is coloured too. */
const CONTAINERS: Record<string, ReadonlyArray<[string, string]>> = {
  acl: [
    ['CAT', 'the categories a permission can name'],
    ['GETUSER', 'what one user may do'],
    ['LIST', 'every user, as the rules that define them'],
    ['SETUSER', 'make or change a user'],
    ['WHOAMI', 'the user this connection authenticated as'],
  ],
  client: [
    ['ID', "this connection's id"],
    ['INFO', 'what the server knows about this connection'],
    ['KILL', 'close somebody else’s connection'],
    ['LIST', 'every connection the server holds'],
    ['NO-EVICT', 'keep this client out of the eviction pool'],
  ],
  cluster: [
    ['INFO', "the cluster's state"],
    ['NODES', 'every node and the slots it holds'],
    ['SLOTS', 'the slot map'],
    ['MYID', "this node's id"],
  ],
  command: [
    ['COUNT', 'how many commands this server has'],
    ['DOCS', 'what a command takes and what it answers'],
    ['INFO', "one command's arity and flags"],
  ],
  config: [
    ['GET', 'read a setting'],
    ['SET', 'change a setting for as long as the server runs'],
    ['REWRITE', 'write the running settings back to the config file'],
    ['RESETSTAT', 'zero the statistics'],
  ],
  function: [
    ['LIST', 'the libraries this server has loaded'],
    ['STATS', 'what the function engine is doing'],
    ['DUMP', 'every library, as bytes'],
  ],
  latency: [
    ['LATEST', 'the worst event of each kind'],
    ['HISTORY', 'one kind of event over time'],
    ['DOCTOR', "the server's own reading of its latency"],
  ],
  memory: [
    ['USAGE', 'what one key costs'],
    ['DOCTOR', "the server's own reading of its memory"],
    ['STATS', 'memory, broken down'],
  ],
  object: [
    ['ENCODING', 'how the value is stored'],
    ['FREQ', 'how often the key is used, under LFU'],
    ['IDLETIME', 'how long since the key was touched'],
    ['REFCOUNT', 'how many references the value has'],
  ],
  pubsub: [
    ['CHANNELS', 'the channels with subscribers'],
    ['NUMSUB', 'how many subscribers a channel has'],
  ],
  script: [
    ['LOAD', 'store a script and answer with its hash'],
    ['EXISTS', 'whether a hash is loaded'],
    ['FLUSH', 'forget every loaded script'],
  ],
  slowlog: [
    ['GET', 'the slowest commands the server kept'],
    ['LEN', 'how many it kept'],
    ['RESET', 'forget them'],
  ],
  xgroup: [
    ['CREATE', 'make a consumer group'],
    ['DESTROY', 'remove one'],
    ['CREATECONSUMER', 'add a consumer to a group'],
  ],
  xinfo: [
    ['STREAM', "the stream's own state"],
    ['GROUPS', 'the consumer groups on it'],
    ['CONSUMERS', 'the consumers of one group'],
  ],
}

/**
 * The commands, with what each of them does. It is deliberately the working set rather than all
 * two hundred and forty: a list nobody can read is a list nobody reads, and anything missing is
 * still typed and still runs — the server is what knows the whole of its own vocabulary.
 */
const COMMANDS: ReadonlyArray<[string, string]> = [
  ['GET', 'the string a key holds'],
  ['SET', 'write a string — takes EX, PX, NX, XX, KEEPTTL'],
  ['SETEX', 'write a string that expires in so many seconds'],
  ['SETNX', 'write a string only if the key is not there'],
  ['GETSET', 'write a string and answer with what was there'],
  ['GETDEL', 'read a string and remove it'],
  ['GETEX', 'read a string and set its expiry in the same breath'],
  ['MGET', 'the strings several keys hold'],
  ['MSET', 'write several strings'],
  ['APPEND', 'add to the end of a string'],
  ['STRLEN', 'how long a string is'],
  ['INCR', 'add one to a number held as a string'],
  ['INCRBY', 'add so much to a number'],
  ['INCRBYFLOAT', 'add so much to a number, with a fraction'],
  ['DECR', 'take one away'],
  ['DECRBY', 'take so much away'],

  ['DEL', 'remove keys, freeing them here and now'],
  ['UNLINK', 'remove keys, freeing them in the background'],
  ['EXISTS', 'how many of these keys are there'],
  ['TYPE', 'what a key holds'],
  ['RENAME', 'move a key, taking whatever name it is given'],
  ['RENAMENX', 'move a key only if the name is free'],
  ['COPY', 'a second key holding the same value'],
  ['KEYS', 'every key matching a glob — reads the whole space'],
  ['SCAN', 'keys a page at a time, which is what a big space needs'],
  ['RANDOMKEY', 'one key, any key'],
  ['DBSIZE', 'how many keys this database holds'],
  ['TTL', 'seconds until a key expires, -1 for never'],
  ['PTTL', 'the same in milliseconds'],
  ['EXPIRE', 'expire a key in so many seconds'],
  ['EXPIREAT', 'expire a key at a moment'],
  ['PERSIST', 'take the expiry off'],
  ['DUMP', 'a key as bytes, for RESTORE'],
  ['RESTORE', 'a key from the bytes DUMP gave'],

  ['HGET', 'one field of a hash'],
  ['HSET', 'write fields of a hash'],
  ['HSETNX', 'write a field only if it is not there'],
  ['HMGET', 'several fields of a hash'],
  ['HGETALL', 'every field and value'],
  ['HDEL', 'remove fields'],
  ['HKEYS', 'the field names'],
  ['HVALS', 'the values'],
  ['HLEN', 'how many fields'],
  ['HEXISTS', 'whether a field is there'],
  ['HINCRBY', 'add so much to a field held as a number'],
  ['HSCAN', 'fields a page at a time'],

  ['LPUSH', 'add to the head of a list'],
  ['RPUSH', 'add to the tail'],
  ['LPOP', 'take from the head'],
  ['RPOP', 'take from the tail'],
  ['LRANGE', 'a window of a list — `0 -1` is all of it'],
  ['LLEN', 'how long a list is'],
  ['LINDEX', 'one element by position'],
  ['LSET', 'replace one element'],
  ['LREM', 'remove elements equal to a value'],
  ['LTRIM', 'keep a window and drop the rest'],
  ['LMOVE', 'move an element from one list to another'],

  ['SADD', 'add members to a set'],
  ['SREM', 'remove members'],
  ['SMEMBERS', 'every member'],
  ['SISMEMBER', 'whether a member is in the set'],
  ['SCARD', 'how many members'],
  ['SPOP', 'take a member out at random'],
  ['SRANDMEMBER', 'a member, without taking it'],
  ['SINTER', 'the members several sets share'],
  ['SUNION', 'the members of several sets together'],
  ['SDIFF', 'the members of the first that are in none of the rest'],
  ['SSCAN', 'members a page at a time'],

  ['ZADD', 'add members with scores'],
  ['ZREM', 'remove members'],
  ['ZSCORE', "one member's score"],
  ['ZINCRBY', 'add so much to a score'],
  ['ZCARD', 'how many members'],
  ['ZCOUNT', 'how many members score within a range'],
  ['ZRANGE', 'members by rank, or by score with BYSCORE'],
  ['ZREVRANGE', 'members by rank, highest first'],
  ['ZRANK', "a member's position"],
  ['ZPOPMIN', 'take the lowest-scoring member'],
  ['ZPOPMAX', 'take the highest'],
  ['ZSCAN', 'members a page at a time'],

  ['XADD', 'append an entry to a stream'],
  ['XRANGE', 'entries between two ids — `- +` is all of them'],
  ['XLEN', 'how many entries'],
  ['XREAD', 'entries after an id, for a reader with no group'],
  ['XDEL', 'remove entries by id'],
  ['XTRIM', 'cut a stream back to a length or an age'],

  ['PFADD', 'add to a HyperLogLog'],
  ['PFCOUNT', 'roughly how many distinct items it has seen'],
  ['GEOADD', 'add points to a geospatial index'],
  ['GEOSEARCH', 'points within a radius or a box'],
  ['BITCOUNT', 'how many bits are set'],
  ['SETBIT', 'set one bit'],
  ['GETBIT', 'read one bit'],

  ['INFO', 'everything the server knows about itself, by section'],
  ['PING', 'whether the server is answering'],
  ['ECHO', 'the server saying back what it was given'],
  ['TIME', "the server's clock"],
  ['LASTSAVE', 'when the dataset was last written to disk'],
  ['BGSAVE', 'write the dataset to disk in the background'],
  ['FLUSHDB', 'every key of this database, gone'],
  ['FLUSHALL', 'every key of every database, gone'],
  ['EVAL', 'run a Lua script'],
  ['EVALSHA', 'run a Lua script already loaded, by hash'],
  ['WAIT', 'block until so many replicas have the writes'],

  // The two-word ones, offered as their first word: the second is completed after it.
  ...Object.keys(CONTAINERS).map(
    (name) => [name.toUpperCase(), 'takes a second word'] as [string, string],
  ),
]

const WORD = /^[^\s"'#;]+/
const NUMBER = /^[+-]?\d+(\.\d+)?$/

/**
 * The colouring, and the whole of it: the verb, a quoted run, a number and a comment. Every other
 * word is a key or a value — bytes the reader chose — and painting those would be inventing a
 * meaning the server does not give them.
 */
const commandStream = StreamLanguage.define<CommandState>({
  name: 'redis',
  startState: () => ({ at: 0, head: '' }),

  token(stream, state) {
    if (stream.sol()) {
      state.at = 0
      state.head = ''
    }
    if (stream.eatSpace()) return null

    if (stream.peek() === '#') {
      stream.skipToEnd()
      return 'comment'
    }
    if (stream.peek() === ';') {
      stream.next()
      state.at = 0
      state.head = ''
      return 'punctuation'
    }

    const quote = stream.peek()
    if (quote === '"' || quote === "'") {
      stream.next()
      let escaped = false
      while (!stream.eol()) {
        const character = stream.next()
        if (escaped) {
          escaped = false
          continue
        }
        if (character === '\\') escaped = true
        else if (character === quote) break
      }
      state.at += 1
      return 'string'
    }

    const word = stream.match(WORD)
    if (!word || word === true) {
      stream.next()
      return null
    }

    const text = word[0]
    const at = state.at
    state.at += 1

    if (at === 0) {
      state.head = text.toLowerCase()
      return 'keyword'
    }
    if (at === 1 && CONTAINERS[state.head]) return 'keyword'
    return NUMBER.test(text) ? 'number' : null
  },
})

/** The verbs, offered where a command begins — which is the start of a line and nowhere else. */
const commandSource: CompletionSource = (context) => {
  const word = context.matchBefore(/^\s*[\w.-]*/)
  if (!word) return null

  const typed = word.text.trimStart()
  if (!context.explicit && typed === '') return null

  return {
    from: word.to - typed.length,
    options: COMMANDS.map(([label, detail]) => ({ label, type: 'keyword', detail })),
    validFor: /^[\w.-]*$/,
  }
}

/** The second word of a container command, which is as much a verb as the first. */
const subcommandSource: CompletionSource = (context) => {
  const before = context.matchBefore(/^\s*([\w]+)\s+([\w-]*)/)
  if (!before) return null

  const parts = /^\s*([\w]+)\s+([\w-]*)$/.exec(before.text)
  if (!parts) return null

  const [, name = '', typed = ''] = parts
  const held = CONTAINERS[name.toLowerCase()]
  if (!held) return null

  return {
    from: before.to - typed.length,
    options: held.map(([label, detail]) => ({ label, type: 'keyword', detail })),
    validFor: /^[\w-]*$/,
  }
}

export function commandLanguage(library: readonly Snippet[]): Extension {
  return [
    commandStream,
    commandStream.data.of({ autocomplete: subcommandSource }),
    commandStream.data.of({ autocomplete: commandSource }),
    // The reader's own, and only the reader's: this app ships no built-in Redis snippets, a command
    // being a verb and its arguments with nothing to lay out. A saved one is offered by name exactly
    // as it is in front of the other engines — see `snippets.ts`.
    commandStream.data.of({ autocomplete: completeFromList(saved(library)) }),
  ]
}
