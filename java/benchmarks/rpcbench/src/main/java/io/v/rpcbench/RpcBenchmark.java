// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.rpcbench;

import com.google.caliper.BeforeExperiment;
import com.google.caliper.Benchmark;
import com.google.caliper.runner.CaliperMain;
import com.google.common.util.concurrent.Futures;
import com.google.common.util.concurrent.ListenableFuture;
import io.v.v23.InputChannelCallback;
import io.v.v23.InputChannels;
import io.v.v23.V;
import io.v.v23.VFutures;
import io.v.v23.context.VContext;
import io.v.v23.naming.Endpoint;
import io.v.v23.rpc.Server;
import io.v.v23.rpc.ServerCall;
import io.v.v23.security.VSecurity;
import io.v.v23.vdl.ServerStream;
import io.v.v23.verror.VException;
import io.v.v23.vom.BinaryDecoder;
import io.v.v23.vom.BinaryEncoder;
import io.v.v23.vom.ConversionException;
import io.v.v23.vom.VomUtil;

import java.io.BufferedInputStream;
import java.io.IOException;
import java.io.PipedInputStream;
import java.io.PipedOutputStream;
import java.util.Arrays;

public class RpcBenchmark {
    VContext baseContext;
    Server echoServer;
    EchoClient echoClient;

    @BeforeExperiment
    public void setUp() throws VException {
        baseContext = V.init();
        VContext serverContext = V.withNewServer(baseContext, "", new EchoServer() {
            @Override
            public ListenableFuture<byte[]> echo(VContext ctx, ServerCall call, byte[] payload) {
                return Futures.immediateFuture(payload);
            }

            @Override
            public ListenableFuture<Void> echoStream(VContext ctx, ServerCall call, final ServerStream<byte[],
                    byte[]> stream) {
                return InputChannels.withCallback(stream, new InputChannelCallback<byte[]>() {
                    @Override
                    public ListenableFuture<Void> onNext(byte[] result) {
                        return stream.send(result);
                    }
                });
            }
        }, VSecurity.newAllowEveryoneAuthorizer());
        echoServer = V.getServer(serverContext);

        Endpoint[] endpoints = echoServer.getStatus().getEndpoints();
        if (endpoints.length == 0) {
            throw new IllegalStateException("No endpoints for server");
        }
        echoClient = EchoClientFactory.getEchoClient("/" + endpoints[0]);
    }

    @Benchmark
    public void echoBenchmark1B(int nreps) throws Exception {
        callEcho(baseContext, echoClient, nreps, 1);
    }

    @Benchmark
    public void echoBenchmark10B(int nreps) throws VException {
        callEcho(baseContext, echoClient, nreps, 10);
    }

    @Benchmark
    public void echoBenchmark100B(int nreps) throws VException {
        callEcho(baseContext, echoClient, nreps, 100);
    }

    @Benchmark
    public void echoBenchmark1KB(int nreps) throws VException {
        callEcho(baseContext, echoClient, nreps, 1000);
    }

    @Benchmark
    public void echoBenchmark10KB(int nreps) throws VException {
        callEcho(baseContext, echoClient, nreps, 10000);
    }

    @Benchmark
    public void echoBenchmark100KB(int nreps) throws VException {
        callEcho(baseContext, echoClient, nreps, 100000);
    }

    @Benchmark
    public void singleEncoder(int nreps) throws IOException, ConversionException {
        PipedOutputStream outStream = new PipedOutputStream();
        PipedInputStream inStream = new PipedInputStream(outStream);
        BinaryEncoder encoder = new BinaryEncoder(outStream);
        BinaryDecoder decoder = new BinaryDecoder(new BufferedInputStream(inStream));
        byte[] payload = new byte[] { 0x01 };
        for (int i = 0; i < nreps; i++) {
            encoder.encodeValue(byte[].class, payload);
            byte[] decoded = (byte[]) decoder.decodeValue(byte[].class);
            if (!Arrays.equals(payload, decoded)) {
                throw new IllegalStateException("unexpected output from decoder");
            }
        }
    }

    @Benchmark
    public void newEncoderEachTime(int nreps) throws VException {
        byte[] payload = new byte[] { 0x01 };
        for (int i = 0; i < nreps; i++) {
            byte[] result = VomUtil.encode(payload, byte[].class);
            byte[] decoded = (byte[]) VomUtil.decode(result, byte[].class);
            if (!Arrays.equals(payload, decoded)) {
                throw new IllegalStateException("unexpected output from decoder");
            }
        }
    }

    private void callEcho(VContext context, EchoClient client, int iterations, int payloadSize) throws VException {
        byte[] payload = new byte[payloadSize];
        for (int i = 0; i < payloadSize; i++) {
            payload[i] = (byte) (i & 0xFF);
        }
        for (int i = 0; i < iterations; i++) {
            VFutures.sync(client.echo(context, payload));
        }
    }

    public static void main(String[] args) throws VException {
        CaliperMain.main(RpcBenchmark.class, args);
    }
}
