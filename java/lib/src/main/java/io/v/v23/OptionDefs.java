// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23;

/**
 * Commonly used options in the Vanadium runtime.
 */
public class OptionDefs {
    /**
     * A key for an option of type {@link io.v.v23.VRuntime} that specifies a runtime
     * implementation.
     */
    public static final String RUNTIME = "io.v.v23.RUNTIME";

    /**
     * A key for an option of type {@link io.v.v23.rpc.Client} that specifies a client.
     */
    public static final String CLIENT = "io.v.v23.CLIENT";

    /**
     * A key for an option of type {@link Boolean} that if provided and {@code true}
     * causes clients to ignore the blessings in remote (server) endpoint during authorization.
     * With this option enabled, clients are susceptible to man-in-the-middle attacks where an
     * imposter server has taken over the network address of a real server.
     */
    public static final String SKIP_SERVER_ENDPOINT_AUTHORIZATION =
            "io.v.v23.SKIP_SERVER_ENDPOINT_AUTHORIZATION";

    /**
     * A key for an option of type {@link io.v.v23.security.Authorizer} that encapsulates the
     * authorization policy used by a client to authorize mounttable servers before sending them a
     * name resolutionrequest. By specifying this policy, clients avoid revealing the names they
     * are interested in resolving to unauthorized mounttables.
     * <p>
     * If no such option is provided, then runtime implementations are expected to
     * default to {@link io.v.v23.security.VSecurity#newEndpointAuthorizer()}.
     */
    public static final String NAME_RESOLUTION_AUTHORIZER = "io.v.v23.NAME_RESOLUTION_AUTHORIZER";

    /**
     * A key for an option of type {@link io.v.v23.security.Authorizer} that encapsulates the
     * authorization policy used by a client to authorize the end server of an RPC.
     * <p>
     * This policy is applied before the client sends information about itself
     * {@code (public key, blessings, the RPC request)} to the server. Thus, if a server
     * does not satisfy this policy then the client will abort the request.
     * <p>
     * Authorization of other servers communicated with in the process of
     * contacting the end server are controlled by other options, like
     * {@link #NAME_RESOLUTION_AUTHORIZER}.
     * <p>
     * Runtime implementations are expected to use
     * {@link io.v.v23.security.VSecurity#newEndpointAuthorizer()}
     * if no explicit server authorizer has been provided for the call.
     */
     public static final String SERVER_AUTHORIZER = "io.v.v23.SERVER_AUTHORIZER";

    /**
     * A key for an option of type {@link String} that specifies the directory that should be
     * used for storing the log files.  If not present, logs will be written into the system's
     * temporary directory.
     */
    public static final String LOG_DIR = "io.v.v23.LOG_DIR";

    /**
     * A key for an option of type {@link Boolean} that specifies whether all logs should be
     * written to standard error instead of files.
     */
    public static final String LOG_TO_STDERR = "io.v.v23.LOG_TO_STDERR";

    /**
     * A key for an option of type {@link Integer} that specifies the level of verbosity for the
     * for the {@code V} logs in the vanadium code.
     */
    public static final String LOG_VLEVEL = "io.v.v23.LOG_VLEVEL";

    /**
     * A key for an option of type {@link String} that specifies the comma-separated list of
     * {@code pattern=N}, where pattern is a literal file name (minus the extension suffix) or
     * a glob pattern, and N is the level of verbosity for the {@code V} logs in the vanadium code.
     * For example:
     * <p><blockquote><pre>
     *     vsync*=5,VRuntime=2
     * </pre></blockquote><p>
     */
    public static final String LOG_VMODULE = "io.v.v23.LOG_VMODULE";

    /**
     * A key for the option of type {@link org.joda.time.Duration} that specifies the time to
     * wait for outstanding server operations to complete when shutting down a server.  Default
     * behavior is to not wait.
     */
    public static final String SERVER_LAME_DUCK_TIMEOUT = "io.v.v23.SERVER_LAME_DUCK_TIMEOUT";
}
