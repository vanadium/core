// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.context.VContext;
import io.v.v23.uniqueid.Id;
import io.v.v23.verror.VException;
import io.v.v23.vom.VomUtil;

import java.lang.reflect.Type;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.locks.ReadWriteLock;
import java.util.concurrent.locks.ReentrantReadWriteLock;

/**
 * A singleton global registry that maps the unique ids of caveats to their validators.
 * <p>
 * It is safe to invoke methods on {@link CaveatRegistry} concurrently.
 */
public class CaveatRegistry {
    private static final Map<Id, RegistryEntry> validators = new HashMap<Id, RegistryEntry>();
    private static final ReadWriteLock lock = new ReentrantReadWriteLock();

    /**
     * Associates a caveat descriptor with the validator that is used for validating
     * all caveats with the same identifier as the descriptor.
     * <p>
     * This method may be called at most once per unique identifier and will throw an exception
     * on duplicate registrations.
     *
     * @param  desc        caveat descriptor
     * @param  validator   caveat validator
     * @throws VException  if the given caveat validator and descriptor couldn't be registered
     */
    public static void register(CaveatDescriptor desc, CaveatValidator validator)
            throws VException {
        String registerer = getRegisterer();
        lock.writeLock().lock();
        RegistryEntry existing = validators.get(desc.getId());
        if (existing != null) {
            lock.writeLock().unlock();
            throw new VException(String.format("Caveat with UUID %s registered twice. " +
                "Once with (%s, validator=%s) from %s, once with (%s, validator=%s) from %s",
                desc.getId(), existing.getDescriptor().getParamType(), existing.getValidator(),
                existing.getRegisterer(), desc.getParamType(), validator, registerer));
        }
        // TODO(spetrovic): Once rogulenko@ is done, get the Type from desc.getParamType().
        Type paramType = null;
        RegistryEntry entry = new RegistryEntry(desc, validator, paramType, registerer);
        validators.put(desc.getId(), entry);
        lock.writeLock().unlock();
    }

    /**
     * Throws an exception iff the restriction encapsulated in the corresponding caveat
     * hasn't been satisfied given the context.
     *
     * @param  context     a vanadium context
     * @param  caveat      security caveat
     * @throws VException  if the caveat couldn't be validated
     */
    public static void validate(VContext context, Call call, Caveat caveat) throws VException {
        RegistryEntry entry = lookup(caveat.getId());
        if (entry == null) {
            throw new CaveatNotRegisteredException(null, caveat.getId());
        }
        Object param = null;
        try {
            // TODO(spetrovic): Once rogulenko@ is done, decode with entry.getParamType().
            param = VomUtil.decode(caveat.getParamVom());
        } catch (VException e) {
            throw new VException(e.getMessage());
        }
        // TODO(spetrovic): Once rogulenko@ is done, pass the type as well.
        entry.validator.validate(context, call, param);
    }

    private static RegistryEntry lookup(Id id) {
        lock.readLock().lock();
        RegistryEntry entry = validators.get(id);
        lock.readLock().unlock();
        return entry;
    }

    private static String getRegisterer() {
        StackTraceElement[] stack = Thread.currentThread().getStackTrace();
        if (stack == null || stack.length < 2) {
            return "";
        }
        StackTraceElement registerer = stack[stack.length - 2];
        return String.format("%s:%d", registerer.getFileName(), registerer.getLineNumber());
    }

    private static class RegistryEntry {
        CaveatDescriptor desc;
        CaveatValidator validator;
        Type paramType;
        String registerer;

        RegistryEntry(CaveatDescriptor desc,
                CaveatValidator validator, Type paramType, String registerer) {
            this.desc = desc;
            this.validator = validator;
            this.paramType = paramType;
            this.registerer = registerer;
        }
        CaveatDescriptor getDescriptor() { return this.desc; }
        CaveatValidator getValidator() { return this.validator; }
        @SuppressWarnings("unused")
        Type getParamType() { return this.paramType; }
        String getRegisterer() { return this.registerer; }
    }

    private CaveatRegistry() {}
}
