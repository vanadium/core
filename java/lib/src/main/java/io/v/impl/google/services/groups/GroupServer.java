// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.services.groups;

import io.v.v23.context.VContext;
import io.v.v23.verror.VException;

/**
 * An implementation of a group server.
 */
public class GroupServer {
    private static native VContext nativeWithNewServer(VContext ctx, Params params)
            throws VException;

    /**
     * Creates a new groups server and attaches it to a new context (which is derived
     * from the provided context).
     * <p>
     * The newly created {@link io.v.v23.rpc.Server} instance can be obtained from the context via
     * {@link io.v.v23.V#getServer}.
     *
     * @param  ctx                           vanadium context
     * @param  params                        group server starting parameters
     * @throws StartException                if there was an error starting the group service
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
     * Parameters used when starting a group service.  Here is an example of a simple
     * parameter creation:
     * <p><blockquote><pre>
     *     GroupServer.Params params = new GroupServer.Params()
     *           .withName("test")
     *           .withStorageEngine(GroupServer.StorageEngine.LEVELDB);
     *     ctx = GroupServer.withNewServer(ctx, params);
     * </pre></blockquote><p>
     *
     * {@link Params} form a tree where derived params are children of the params from
     * which they were derived.  Children inherit all the properties of their parent except for the
     * property being replaced (the name/statsPrefix in the example above).
     */
    public static class Params {
        private Params parent = null;

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
         * Returns a name that the service will mount itself on.
         */
        public String getName() {
            if (this.name != null) return this.name;
            if (this.parent != null) return this.parent.getName();
            return null;
        }

        /**
         * Returns a directory used for persisting the groups.
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
     * Storage engine used for storing the groups data.
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

    /**
     * Exception thrown if the group server couldn't be started.
     */
    public static class StartException extends Exception {
        public StartException(String msg) {
            super(msg);
        }
    }
}
