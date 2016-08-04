// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package io.v.impl.google.services.mounttable;

import io.v.v23.context.VContext;
import io.v.v23.security.access.Permissions;
import io.v.v23.verror.VException;

import java.util.Map;

/**
 * An implementation of a mounttable server.
 */
public class MountTableServer {
    private static native VContext nativeWithNewServer(VContext ctx, Params params)
            throws VException;

    /**
     * Creates a new mounttable server and attaches it to a new context (which is derived
     * from the provided context).
     * <p>
     * The newly created {@link io.v.v23.rpc.Server} instance can be obtained from the context via
     * {@link io.v.v23.V#getServer}.
     *
     * @param  ctx                           vanadium context
     * @param  params                        mounttable starting parameters
     * @throws StartException                if there was an error starting the mounttable service
     * @return                               a child context to which the new server is attached
     */
    public static VContext withNewServer(VContext ctx, Params params) throws StartException {
        try {
            return nativeWithNewServer(ctx, params);
        } catch (VException e) {
            throw new StartException(e.getMessage());
        }
    }

    /**
     * Parameters used when starting a mounttable service.  Here is an example of a simple
     * parameter creation:
     * <p><blockquote><pre>
     *     MountTableServer.Params params = new MountTableServer.Params()
     *           .withName("test")
     *           .withStatsPrefix("test");
     *     ctx = MountTableServer.withNewServer(ctx, params);
     * </pre></blockquote><p>
     *
     * {@link Params} form a tree where derived params are children of the params from
     * which they were derived.  Children inherit all the properties of their parent except for the
     * property being replaced (the name/statsPrefix in the example above).
     */
    public static class Params {
        private Params parent = null;

        private String name;
        private String rootDir;
        private Map<String, Permissions> perms;
        private String statsPrefix;

        /**
         * Creates a new (and empty) {@link Params} object.
         */
        public Params() {
        }

        private Params(Params parent) {
            this.parent = parent;
        }

        /**
         * Returns a child of the current params with the given mount name.
         */
        public Params withName(String name) {
            Params ret = new Params(this);
            ret.name = name;
            return ret;
        }

        /**
         * Returns a child of the current params with the given storage root directory.
         */
        public Params withStorageRootDir(String rootDir) {
            Params ret = new Params(this);
            ret.rootDir = rootDir;
            return ret;
        }

        /**
         * Returns a child of the current params with the given map from paths in the mounttable
         * to permissions.
         */
        public Params withPermissions(Map<String, Permissions> perms) {
            Params ret = new Params(this);
            ret.perms = perms;
            return ret;
        }

        /**
         * Returns a child of the current params with the given stats name prefix.
         */
        public Params withStatsPrefix(String statsPrefix) {
            Params ret = new Params(this);
            ret.statsPrefix = statsPrefix;
            return ret;
        }

        /**
         * Returns a name that the service will mount itself on.
         */
        public String getName() {
            if (this.name != null) return this.name;
            if (this.parent != null) return this.parent.getName();
            return null;
        }

        /**
         * Returns a directory used for persisting the mounttable.
         */
        public String getStorageRootDir() {
            if (this.rootDir != null) return this.rootDir;
            if (this.parent != null) return this.parent.getStorageRootDir();
            return null;
        }

        /**
         * Returns a map from paths in the mounttable to the {@link Permissions} for that path.
         */
        public Map<String, Permissions> getPermissions() {
            if (this.perms != null) return this.perms;
            if (this.parent != null) return this.parent.getPermissions();
            return null;
        }

        /**
         * Returns a stats name prefix.
         */
        public String getStatsPrefix() {
            if (this.statsPrefix != null) return this.statsPrefix;
            if (this.parent != null) return this.parent.getStatsPrefix();
            return null;
        }
    }

    /**
     * Exception thrown if the mounttable server couldn't be started.
     */
    public static class StartException extends Exception {
        public StartException(String msg) {
            super(msg);
        }
    }
}
