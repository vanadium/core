// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.services.syncbase;

import io.v.v23.context.VContext;
import io.v.v23.security.access.Permissions;
import io.v.v23.verror.VException;

/**
 * An implementation of a syncbase server.
 */
public class SyncbaseServer {
    private static native VContext nativeWithNewServer(VContext ctx, Params params)
            throws VException;

    /**
     * Creates a new syncbase server and attaches it to a new context (which is derived
     * from the provided context).
     * <p>
     * The newly created {@link io.v.v23.rpc.Server} instance can be obtained from the context via
     * {@link io.v.v23.V#getServer}.
     *
     * @param  ctx                           vanadium context
     * @param  params                        syncbase starting parameters
     * @throws StartException                if there was an error starting the syncbase service
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
     * Parameters used when starting a syncbase service.  Here is an example of a simple
     * parameter creation:
     * <p><blockquote><pre>
     *     SyncbaseServer.Params params = new SyncbaseServer.Params()
     *           .withName("test")
     *           .withStorageEngine(SyncbaseServer.StorageEngine.LEVELDB);
     *     ctx = SyncbaseServer.withNewServer(ctx, params);
     * </pre></blockquote><p>
     *
     * {@link Params} form a tree where derived params are children of the params from
     * which they were derived.  Children inherit all the properties of their parent except for the
     * property being replaced (the name/storageEngine in the example above).
     */
    public static class Params {
        private Params parent = null;

        private Permissions permissions;
        private String name;
        private String storageRootDir;
        private StorageEngine storageEngine;

        /**
         * Creates a new (and empty) {@link Params} object.
         */
        public Params() {
        }

        private Params(Params parent) {
            this.parent = parent;
        }

        /**
         * Returns a child of the current params with the given permissions.
         */
        public Params withPermissions(Permissions permissions) {
            Params ret = new Params(this);
            ret.permissions = permissions;
            return ret;
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
            ret.storageRootDir = rootDir;
            return ret;
        }

        /**
         * Returns a child of the current params with the given storage engine.
         */
        public Params withStorageEngine(StorageEngine engine) {
            Params ret = new Params(this);
            ret.storageEngine = engine;
            return ret;
        }

        /**
         * Returns permissions that the syncbase service will be started with.
         */
        public Permissions getPermissions() {
            if (this.permissions != null) return this.permissions;
            if (this.parent != null) return this.parent.getPermissions();
            return null;
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
         * Returns a root directory for all of the service's storage files.
         */
        public String getStorageRootDir() {
            if (this.storageRootDir != null) return this.storageRootDir;
            if (this.parent != null) return this.parent.getStorageRootDir();
            return null;
        }

        /**
         * Returns a storage engine for the service.
         */
        public StorageEngine getStorageEngine() {
            if (this.storageEngine != null) return this.storageEngine;
            if (this.parent != null) return this.parent.getStorageEngine();
            return null;
        }
    }

    /**
     * Exception thrown if the syncbase server couldn't be started.
     */
    public static class StartException extends Exception {
        public StartException(String msg) {
            super(msg);
        }
    }

    /**
     * Storage engine used for storing the syncbase data.
     */
    public enum StorageEngine {
        LEVELDB   ("leveldb"),
        MEMSTORE  ("memstore");

        private final String value;

        StorageEngine(String value) {
            this.value = value;
        }

        /**
         * Returns the {@link String} value corresponding to this {@link StorageEngine}.
         */
        public String getValue() {
            return this.value;
        }
    }
}
