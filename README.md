# Fleet Document Expiry Vault

**Vehicle document expiry tracker for small transport fleets.** A single Go binary
that keeps every RC, insurance policy, fitness certificate, permit, PUC
certificate, road tax receipt and driving licence in one place on your own
machine, and answers the question that matters before the vehicle leaves the
yard: **can this vehicle run today?**

One expired document makes a vehicle illegal to run and voids its insurance.
Most small fleets track this on paper and find out at a checkpoint.

## What it does

- **Can this vehicle run today?** One vehicle, one answer. When the answer is
  no, it names exactly which documents are in the way and how many days each is
  overdue.
- **Fleet board sorted by urgency** — every document across every vehicle and
  driver, expired first (longest overdue at the top), then soonest to expire, in
  red, amber and green.
- **Three reminder tiers** — 30, 15 and 3 days before expiry.
- **Driver licence list** — a licence belongs to the person, not the lorry.
- **Scan references** — record where each certificate is kept on your own
  machine. No file is ever uploaded.
- **Suggested expiry dates** from the issue date and the usual validity period,
  clamped to month ends, always yours to overrule.
- **Printable renewal checklists** — what to take to the RTO, the testing
  station or the insurer for each document.
- **CSV import and export** of the whole book, so your records are never locked
  inside the tool.

## Who it's for

Small fleet owners and transport contractors in India running anything from two
vehicles to a couple of dozen. It is not a telematics system and does not track
where your vehicles are. It tracks paper.

## The rule it implements

Under the **Motor Vehicles Act, 1988** and the **Central Motor Vehicles Rules,
1989**, a vehicle on a public road must be able to produce a valid set of
documents. Four of them stop the vehicle outright when they lapse, and the tool
blocks on them by default:

| Document | Why it stops the vehicle |
| --- | --- |
| Insurance | Third-party cover is compulsory under section 146 of the Act. An expired policy means any claim falls on you personally. |
| Certificate of fitness (CF) | Required for a transport vehicle under section 56. |
| Permit | Required under section 66 for a goods or passenger vehicle used for hire or reward. |
| PUC certificate | Required under rule 115 of the CMVR. |

**Registration (RC)** and **road tax** are recorded and reported but do not stop
the vehicle by default — an expired RC or an unpaid tax demand is a penalty and
a renewal job rather than a roadside immobilisation. All six are switches you
can change, because vehicle classes differ: a carriage exempt from a permit
under section 66 should not be grounded by a missing one.

A document counts as valid through the whole of its expiry date — "valid upto
31-03-2027" includes 31 March — so the expiry day itself is amber and the
morning after is red.

## Install and run

One binary, no dependencies, no installer. With Go 1.25 or newer:

```sh
git clone https://github.com/sreenivas-sadhu-prabhakara/fleet-doc-expiry-vault
cd fleet-doc-expiry-vault

# run it straight away
go run .

# or build the binary once and keep it
go build -o fleet-doc-expiry-vault .
./fleet-doc-expiry-vault
```

Then open <http://127.0.0.1:8080> in a browser on the same machine.

| Flag | Default | What it does |
| --- | --- | --- |
| `-addr` | `127.0.0.1:8080` | Address to listen on. Loopback only unless you deliberately change it. |
| `-data` | `./fleet-doc-expiry-vault-data.json` | Path to the local data file. |

Stop it with Ctrl-C.

## Where your data is stored

In one JSON file, wherever `-data` points. It is plain text you can open in any
editor, and it is written atomically — to a temporary file that is then renamed
over the old one — so a crash or a full disk leaves your previous good file
intact rather than a half-written one. The file is created with owner-only
permissions.

That file is the whole system. Back it up by copying it; move it to another
machine by copying it. There is no account, no database, no sync and no cloud.

The binary makes **no outbound network connections of any kind**: no telemetry,
no analytics, no update check, no fonts or scripts fetched from anywhere. The
one socket it opens is the local one it serves the interface on.

## Indicative validity periods

The tool can suggest an expiry date from an issue date. Those suggestions come
from one table of named constants in `internal/fleet/validity.go`, every one of
which is overridable from the Data and settings screen:

| Document | First issue | On renewal |
| --- | --- | --- |
| PUC certificate | 12 months (new vehicle) | 6 months |
| Insurance | 12 months | 12 months |
| Certificate of fitness | 24 months (new transport vehicle) | 12 months |
| Permit | 60 months | 60 months |
| Registration, road tax, driving licence | no default | no default |

## Scope and accuracy

**These figures are indicative, and the date on your paper always wins.**
Statutory validity periods vary by vehicle class, by the age of the vehicle, by
permit type and by state, and they change when the rules are amended. Treat
every suggested date as a prompt to check the certificate, never as a substitute
for reading it, and confirm the current requirement with your RTO or state
transport department before you act on it. The renewal checklists are the papers
a small fleet is usually asked for; the authority is the authority.

**There is no connection to any government system.** This tool does not talk to
VAHAN, Parivahan or SARATHI, and makes no outbound network calls at all. Nothing
you enter is verified against any official record. The dates in your vault are
whatever you typed from your own papers, so a date typed wrongly stays wrong
until you correct it. The official portal addresses are listed inside the tool
as plain text for you to copy into your own browser.

Last, this is a record-keeping tool, not legal advice. It tells you what your
own records say about a vehicle. It cannot tell you that your records are
complete.

## Development

```sh
gofmt -l .              # must print nothing
go vet ./...
go test ./... -count=1
go build -o /tmp/fleet-doc-expiry-vault-bin .
```

The rules live in `internal/fleet` as pure functions over plain values, with
table-driven tests covering every tier boundary, the leap-day arithmetic, the
month-end clamping and each blocking combination. The HTTP layer in `server.go`
holds no rules of its own.

MIT licensed.
