// Package codedist holds the code distribution logic: on the student side, the
// Board of snippets shown in the overlay and the Receiver that applies the
// console's code.share, code.clear and msg.show messages and builds their acks.
//
// It does not touch the network, the clipboard or the UI: the app wires those.
package codedist
