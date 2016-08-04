// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23;

import com.google.common.base.Preconditions;
import com.google.common.collect.AbstractIterator;
import com.google.common.util.concurrent.AsyncFunction;
import com.google.common.util.concurrent.FutureCallback;
import com.google.common.util.concurrent.Futures;
import com.google.common.util.concurrent.ListenableFuture;
import com.google.common.util.concurrent.MoreExecutors;
import com.google.common.util.concurrent.SettableFuture;

import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.concurrent.Executor;

import javax.annotation.CheckReturnValue;

import io.v.v23.context.VContext;
import io.v.v23.verror.EndOfFileException;
import io.v.v23.verror.VException;

import static io.v.v23.VFutures.sync;

/**
 * Contains static utility methods that operate on or return objects of type {@link InputChannel}.
 */
public class InputChannels {
    /**
     * Function used for transforming an input value into an output value.
     */
    public interface TransformFunction<F, T> {
        /**
         * Returns the result of transforming {@code from}, or {@code null} if the value should
         * be skipped.
         *
         * @throws VException if there was a transform error
         */
        T apply(F from) throws VException;
    }

    /**
     * Returns an {@link InputChannel} that applies the provided {@code function} to each element
     * of {@code fromChannel}.
     */
    public static <F,T> InputChannel<T> transform(
            VContext ctx, InputChannel<F> fromChannel,
            TransformFunction<? super F, ? extends T> function) {
        return new TransformedChannel<>(ctx, fromChannel, function);
    }

    /**
     * Returns a new {@link ListenableFuture} whose result is the list of all elements received
     * from the provided {@link InputChannel}.
     * <p>
     * The returned future will be executed on a
     * {@link MoreExecutors#directExecutor() direct executor}.
     */
    @CheckReturnValue
    public static <T> ListenableFuture<List<T>> asList(final InputChannel<T> channel) {
        return asList(channel, MoreExecutors.directExecutor());
    }

    /**
     * Returns a new {@link ListenableFuture} whose result is the list of all elements received
     * from the provided {@link InputChannel}.
     * <p>
     * The returned future will be executed on the provided {@code executor}.
     */
    @CheckReturnValue
    public static <T> ListenableFuture<List<T>> asList(final InputChannel<T> channel,
                                                       Executor executor) {
        final SettableFuture<List<T>> future = SettableFuture.create();
        Futures.addCallback(channel.recv(), newCallbackForList(
                channel, new ArrayList<T>(), future, executor), executor);
        return future;
    }

    private static <T> FutureCallback<T> newCallbackForList(
            final InputChannel<T> channel, final List<T> list,
            final SettableFuture<List<T>> future,
            final Executor executor) {
        return new FutureCallback<T>() {
            @Override
            public void onSuccess(T result) {
                list.add(result);
                Futures.addCallback(channel.recv(),
                        newCallbackForList(channel, list, future, executor), executor);
            }
            @Override
            public void onFailure(Throwable t) {
                if (t instanceof EndOfFileException) {
                    future.set(list);
                } else {
                    future.setException(t);
                }
            }
        };
    }

    /**
     * Returns a new {@link ListenableFuture} whose result is available when the provided
     * {@link InputChannel} has exhausted all of its elements.
     * <p>
     * The returned future will be executed on a
     * {@link MoreExecutors#directExecutor() direct executor}.
     */
    @CheckReturnValue
    public static <T> ListenableFuture<Void> asDone(final InputChannel<T> channel) {
        return asDone(channel, MoreExecutors.directExecutor());
    }

    /**
     * Returns a new {@link ListenableFuture} whose result is available when the provided
     * {@link InputChannel} has exhausted all of its elements.
     * <p>
     * The returned future will be executed on the provided {@code executor}.
     */
    @CheckReturnValue
    public static <T> ListenableFuture<Void> asDone(final InputChannel<T> channel,
                                                    Executor executor) {
        final SettableFuture<Void> future = SettableFuture.create();
        Futures.addCallback(channel.recv(),
                newCallbackForDone(channel, future, executor), executor);
        return future;
    }

    private static <T> FutureCallback<T> newCallbackForDone(final InputChannel<T> channel,
                                                            final SettableFuture<Void> future,
                                                            final Executor executor) {
        return new FutureCallback<T>() {
            @Override
            public void onSuccess(T result) {
                Futures.addCallback(channel.recv(),
                        newCallbackForDone(channel, future, executor), executor);
            }
            @Override
            public void onFailure(Throwable t) {
                if (t instanceof EndOfFileException) {
                    future.set(null);
                } else {
                    future.setException(t);
                }
            }
        };
    }

    /**
     * Returns a {@link VIterable} over all the elements in {@code channel}, blocking in every
     * iteration.
     * <p>
     * The returned iterator will terminate gracefully iff {@code channel}'s
     * {@link InputChannel#recv} call fails with a {@link io.v.v23.verror.EndOfFileException}.
     */
    public static <T> VIterable<T> asIterable(InputChannel<? extends T> channel) {
        return new ChannelIterable<>(channel);
    }

    /**
     * Iterates over all elements in {@code channel}, invoking {@link InputChannelCallback#onNext}
     * method on the provided callback for each element.
     * <p>
     * Returns a new {@link ListenableFuture} that completes when the provided {@link InputChannel}
     * has exhausted all of its elements or has encountered an error.
     * <p>
     * The returned future and all the callbacks will be executed on a
     * {@link MoreExecutors#directExecutor() direct executor}.
     */
    @CheckReturnValue
    public static <T> ListenableFuture<Void> withCallback(
            InputChannel<T> channel, InputChannelCallback<? super T> callback) {
        return withCallback(channel, callback, MoreExecutors.directExecutor());
    }

    /**
     * Iterates over all elements in {@code channel}, invoking {@link InputChannelCallback#onNext}
     * method on the provided callback for each element.
     * <p>
     * Returns a new {@link ListenableFuture} that completes when the provided {@link InputChannel}
     * has exhausted all of its elements or has encountered an error.
     * <p>
     * The returned future and all the callbacks will be executed on the provided {@code executor}.
     */
    @CheckReturnValue
    public static <T> ListenableFuture<Void> withCallback(
            InputChannel<T> channel,
            InputChannelCallback<? super T> callback,
            Executor executor) {
        final SettableFuture<Void> future = SettableFuture.create();
        Futures.addCallback(channel.recv(),
                newCallbackForCallback(channel, future, callback, executor), executor);
        return future;
    }

    private static <T> FutureCallback<T> newCallbackForCallback(
            final InputChannel<T> channel, final SettableFuture<Void> future,
            final InputChannelCallback<? super T> callback, final Executor executor) {
        return new FutureCallback<T>() {
            @Override
            public void onSuccess(T result) {
                ListenableFuture<Void> done = callback.onNext(result);
                if (done == null) {
                    done = Futures.immediateFuture(null);
                }
                Futures.addCallback(Futures.transform(done, new AsyncFunction<Void, T>() {
                            @Override
                            public ListenableFuture<T> apply(Void input) throws Exception {
                                return channel.recv();
                            }
                        }),
                        newCallbackForCallback(channel, future, callback, executor), executor);
            }
            @Override
            public void onFailure(Throwable t) {
                if (t instanceof EndOfFileException) {
                    future.set(null);
                    return;
                }
                future.setException(t);
            }
        };
    }

    private static class TransformedChannel<F, T> implements InputChannel<T> {
        private final VContext ctx;
        private final InputChannel<F> fromChannel;
        private final TransformFunction<? super F, ? extends T> function;

        private TransformedChannel(VContext ctx,
                                   InputChannel<F> fromChannel,
                                   TransformFunction<? super F, ? extends T> function) {
            this.ctx = ctx;
            this.fromChannel = fromChannel;
            this.function = function;
        }
        @Override
        public ListenableFuture<T> recv() {
            return VFutures.withUserLandChecks(ctx,
                    Futures.transform(fromChannel.recv(), new AsyncFunction<F, T>() {
                @Override
                public ListenableFuture<T> apply(F input) throws Exception {
                    T output = function.apply(input);
                    if (output == null) {
                        return recv();
                    }
                    return Futures.immediateFuture(output);
                }
            }));
        }
    }

    private static class ChannelIterable<T> implements VIterable<T> {
        private final InputChannel<? extends T> fromChannel;
        private boolean isCreated;
        private volatile VException error;

        private ChannelIterable(InputChannel<? extends T> fromChannel) {
            this.fromChannel = fromChannel;
        }

        public synchronized Iterator<T> iterator() {
            Preconditions.checkState(!isCreated, "Can only create one iterator.");
            isCreated = true;
            return new AbstractIterator<T>() {
                protected T computeNext() {
                    try {
                        T result = sync(fromChannel.recv());
                        return result;
                    } catch (EndOfFileException e) {
                        return endOfData();
                    } catch (VException e) {
                        error = e;
                        return endOfData();
                    }
                }
            };
        }

        public VException error() {
            return error != null ? error : null;
        }
    }
}
