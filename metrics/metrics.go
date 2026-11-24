package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	LabelChain   = "chain"
	LabelAddress = "address"
	LabelNetwork = "network"
	LabelStatus  = "status"
)

var (
	VMVotesCounter = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vm_votes_total",
			Help: "Number of VM votes(amplifier)",
		},
		[]string{LabelChain, LabelAddress, LabelNetwork, LabelStatus},
	)

	EVMVotesCounter = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "evm_votes_total",
			Help: "Number of EVM votes",
		},
		[]string{LabelChain, LabelAddress, LabelNetwork, LabelStatus},
	)

	MaintainersGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "maintainers_status_list",
			Help: "Maintainer's status, 1 is active, 0 is not active",
		},
		[]string{LabelChain, LabelAddress, LabelNetwork},
	)

	HeartbeatsCounter = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "heartbeats_total",
			Help: "Heartbeat of the application",
		},
		[]string{LabelChain, LabelAddress, LabelStatus},
	)
)

// --- EVM Votes ---

func SetVMVotes(chain, address, network string, status string, count int) {
	VMVotesCounter.With(prometheus.Labels{
		LabelChain:   chain,
		LabelAddress: address,
		LabelNetwork: network,
		LabelStatus:  status,
	}).Set(float64(count))
}

func SetVMVotesMissed(chain, address, network string, missCnt int) {
	SetVMVotes(chain, address, network, "missed", missCnt)
}

func SetVMVotesSuccess(chain, address, network string, successCnt int) {
	SetVMVotes(chain, address, network, "success", successCnt)
}

// --- EVM Votes ---

func SetEVMVotes(chain, address, network string, status string, count int) {
	EVMVotesCounter.With(prometheus.Labels{
		LabelChain:   chain,
		LabelAddress: address,
		LabelNetwork: network,
		LabelStatus:  status,
	}).Set(float64(count))
}

func SetEVMVotesMissed(chain, address, network string, missCnt int) {
	SetEVMVotes(chain, address, network, "missed", missCnt)
}

func SetEVMVotesSuccess(chain, address, network string, successCnt int) {
	SetEVMVotes(chain, address, network, "success", successCnt)
}

// --- Maintainers ---

func SetMaintainersStatus(chain, address, network string, status int) {
	MaintainersGauge.With(prometheus.Labels{
		LabelChain:   chain,
		LabelAddress: address,
		LabelNetwork: network,
	}).Set(float64(status))
}

// --- Heartbeats ---

func SetHeartbeats(chain, address, status string, count int) {
	HeartbeatsCounter.With(prometheus.Labels{
		LabelChain:   chain,
		LabelAddress: address,
		LabelStatus:  status,
	}).Set(float64(count))
}

func SetHeartbeatsCounterMissed(chain, address string, missCnt int) {
	SetHeartbeats(chain, address, "missed", missCnt)
}

func SetHeartbeatsCounterSuccess(chain, address string, successCnt int) {
	SetHeartbeats(chain, address, "success", successCnt)
}

// --- Poll results, signings and check freshness of the monitored account ---

const (
	LabelKind   = "kind"
	LabelResult = "result"
	LabelCheck  = "check"
)

var (
	// PollVotesGauge counts the polls of the check window by how the monitored
	// account behaved in them. Unlike vm_votes_total and evm_votes_total it
	// compares each vote with the outcome of its poll.
	PollVotesGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "axelmon_poll_votes",
			Help: "Polls in the check window by result for the monitored account: agreed, minority, not_voted, late, undecided",
		},
		[]string{LabelNetwork, LabelKind, LabelChain, LabelAddress, LabelResult},
	)

	SigningsGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "axelmon_signings",
			Help: "Signing sessions in the check window by result for the monitored account: signed, not_signed",
		},
		[]string{LabelNetwork, LabelChain, LabelAddress, LabelResult},
	)

	LastCheckGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "axelmon_last_check_timestamp_seconds",
			Help: "Unix time of the last check that read all its chains without an error",
		},
		[]string{LabelNetwork, LabelCheck},
	)
)

func SetPollVotes(network, kind, chain, address, result string, count int) {
	PollVotesGauge.With(prometheus.Labels{
		LabelNetwork: network,
		LabelKind:    kind,
		LabelChain:   chain,
		LabelAddress: address,
		LabelResult:  result,
	}).Set(float64(count))
}

func SetSignings(network, chain, address, result string, count int) {
	SigningsGauge.With(prometheus.Labels{
		LabelNetwork: network,
		LabelChain:   chain,
		LabelAddress: address,
		LabelResult:  result,
	}).Set(float64(count))
}

func SetLastCheck(network, check string) {
	LastCheckGauge.With(prometheus.Labels{
		LabelNetwork: network,
		LabelCheck:   check,
	}).SetToCurrentTime()
}
