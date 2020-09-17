package conn

import "v.io/v23/verror"

var (
	ErrMissingSetupOption       = verror.NewID("MissingSetupOption")                                      //verror.NoRetry, "{1:}{2:} missing required setup option{:3}.")
	ErrUnexpectedMsg            = verror.NewID("UnexpectedMsg")                                           //verror.NoRetry, "{1:}{2:} unexpected message type{:3}.")
	ErrConnectionClosed         = verror.NewID("ConnectionClosed")                                        //verror.NoRetry, "{1:}{2:} connection closed.")
	ErrRemoteError              = verror.NewID("RemoteError")                                             //verror.NoRetry, "{1:}{2:} remote end received err{:3}.")
	ErrSend                     = verror.NewID("Send")                                                    //verror.NoRetry, "{1:}{2:} failure sending {3} message to {4}{:5}.")
	ErrRecv                     = verror.NewID("Recv")                                                    //verror.NoRetry, "{1:}{2:} error reading from {3}{:4}")
	ErrCounterOverflow          = verror.NewID("CounterOverflow")                                         //verror.NoRetry, "{1:}{2:} A remote process has sent more data than allowed.")
	ErrBlessingsFlowClosed      = verror.NewID("BlessingsFlowClosed")                                     //verror.NoRetry, "{1:}{2:} The blessings flow was closed with error{:3}.")
	ErrInvalidChannelBinding    = verror.NewID("InvalidChannelBinding")                                   //verror.NoRetry, "{1:}{2:} The channel binding was invalid.")
	ErrNoPublicKey              = verror.NewID("NoPublicKey")                                             //verror.NoRetry, "{1:}{2:} No public key was received by the remote end.")
	ErrDialingNonServer         = verror.NewID("DialingNonServer")                                        //verror.NoRetry, "{1:}{2:} You are attempting to dial on a connection with no remote server: {:3}.")
	ErrAcceptorBlessingsMissing = verror.NewID("AcceptorBlessingsMissing")                                //verror.NoRetry, "{1:}{2:} The acceptor did not send blessings.")
	ErrDialerBlessingsMissing   = verror.NewID("DialerBlessingsMissing")                                  //verror.NoRetry, "{1:}{2:} The dialer did not send blessings.")
	ErrBlessingsNotBound        = verror.NewID("BlessingsNotBound")                                       //verror.NoRetry, "{1:}{2:} blessings not bound to connection remote public key")
	ErrInvalidPeerFlow          = verror.NewID("InvalidPeerFlow")                                         //verror.NoRetry, "{1:}{2:} peer has chosen flow id from local domain.")
	ErrChannelTimeout           = verror.NewID("ChannelTimeout")                                          //verror.NoRetry, "{1:}{2:} the channel has become unresponsive.")
	ErrCannotDecryptBlessings   = verror.NewID("CannotDecryptBlessings")                                  //verror.NoRetry, "{1:}{2:} cannot decrypt the encrypted blessings sent by peer{:3}")
	ErrCannotDecryptDischarges  = verror.NewID("CannotDecryptDischarges")                                 //verror.NoRetry, "{1:}{2:} cannot decrypt the encrypted discharges sent by peer{:3}")
	ErrCannotEncryptBlessings   = verror.NewID("CannotEncryptBlessings")                                  //verror.NoRetry, "{1:}{2:} cannot encrypt blessings for peer {3}{:4}")
	ErrCannotEncryptDischarges  = verror.NewID("CannotEncryptDischarges")                                 //verror.NoRetry, "{1:}{2:} cannot encrypt discharges for peers {3}{:4}")
	ErrNoCrypter                = verror.Register("v.io/x/ref/runtime/internal/flow/conn.NoCrypter")      //verror.NoRetry, "{1:}{2:} no blessings-based crypter available")
	ErrNoPrivateKey             = verror.Register("v.io/x/ref/runtime/internal/flow/conn.NoPrivateKey")   //verror.NoRetry, "{1:}{2:} no blessings private key available for decryption")
	ErrIdleConnKilled           = verror.Register("v.io/x/ref/runtime/internal/flow/conn.IdleConnKilled") //verror.NoRetry, "{1:}{2:} Connection killed because idle expiry was reached.")
)
