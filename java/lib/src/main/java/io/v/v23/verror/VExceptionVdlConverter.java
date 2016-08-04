// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.verror;

import io.v.v23.vdl.NativeTypes.Converter;
import io.v.v23.vdl.Types;
import io.v.v23.vdl.VdlAny;
import io.v.v23.vdl.VdlType;
import io.v.v23.vdl.VdlValue;
import io.v.v23.vdl.WireError;
import io.v.v23.vdl.WireRetryCode;
import io.v.v23.verror.VException.ActionCode;

import java.lang.reflect.Constructor;
import java.util.ArrayList;
import java.util.List;

/**
 * Converts {@code VException} to its VDL wire type and vice-versa.
 */
public final class VExceptionVdlConverter extends Converter {
    public static final VExceptionVdlConverter INSTANCE = new VExceptionVdlConverter();

    private VExceptionVdlConverter() {
        super(WireError.class);
    }

    private WireRetryCode actionCodeToWire(ActionCode code) {
        switch (code) {
            case NO_RETRY: return WireRetryCode.NoRetry;
            case RETRY_CONNECTION: return WireRetryCode.RetryConnection;
            case RETRY_REFETCH: return WireRetryCode.RetryRefetch;
            case RETRY_BACKOFF: return WireRetryCode.RetryBackoff;
            default: return WireRetryCode.NoRetry;
        }
    }

    @Override
    public WireError vdlValueFromNative(Object nativeValue) {
        assertInstanceOf(nativeValue, VException.class);
        VException e = (VException) nativeValue;
        List<VdlAny> paramVals = new ArrayList<VdlAny>();
        Object[] params = e.getParams();
        VdlType[] paramTypes = e.getParamTypes();
        for (int i = 0; i < params.length; ++i) {
            if (paramTypes[i] == null) {
                continue;  // dropping the param.
            }
            paramVals.add(new VdlAny(paramTypes[i], params[i]));
        }
        return new WireError(e.getID(), actionCodeToWire(e.getAction()), e.getMessage(), paramVals);
    }

    @Override
    public Object nativeFromVdlValue(VdlValue value) {
        assertInstanceOf(value, WireError.class);
        WireError error = (WireError) value;
        VException.IDAction idAction = new VException.IDAction(error.getId(),
                VException.ActionCode.fromValue(error.getRetryCode().ordinal()));

        List<VdlAny> paramVals = error.getParamList();
        Object[] params = new Object[paramVals.size()];
        VdlType[] paramTypes = new VdlType[paramVals.size()];
        for (int i = 0; i < paramVals.size(); ++i) {
            VdlAny paramVal = paramVals.get(i);
            params[i] = paramVal.getElem();
            paramTypes[i] = paramVal.getElemType();
        }
        VException v = new VException(idAction, error.getMsg(), params, paramTypes);
        // See if a subclass can handle further conversion.
        String path = classPath(v.getID());
        Class<?> c = Types.loadClassForVdlName(path);
        if (c == null) {
            c = Types.loadClassForVdlName(path + "Exception");
        }
        if (c != null) {
            try {
                Constructor constructor = c.getDeclaredConstructor(VException.class);
                if (!constructor.isAccessible()) {
                    constructor.setAccessible(true);
                }
                return constructor.newInstance(v);
            } catch (Exception e) {}
        }
        return v;
    }

    private static String classPath(String errId) {
        int idx = errId.lastIndexOf(".");
        if (idx < 0) {
            return errId;
        }
        String errName = errId.substring(idx + 1);
        errName = Character.toUpperCase(errName.charAt(0)) + errName.substring(1);
        return errId.substring(0, idx) + "/" + errName;
    }
}
