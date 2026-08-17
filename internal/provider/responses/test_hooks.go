//go:build profile_public

package responses

var sendChunkEnterBlocking func()

func notifySendChunkEnterBlocking() {
	if sendChunkEnterBlocking != nil {
		sendChunkEnterBlocking()
	}
}
