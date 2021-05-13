package scripting

import (
	"fmt"
	"strings"
	"time"

	"v.io/v23/security"
	"v.io/x/ref/cmd/principal/caveatflag"
	"v.io/x/ref/lib/slang"
)

func parseTime(rt slang.Runtime, layout, value string) (time.Time, error) {
	return time.Parse(layout, value)
}

func deadline(rt slang.Runtime, when time.Time, expiry time.Duration) (time.Time, error) {
	return when.Add(expiry), nil
}

func localTime(rt slang.Runtime) (time.Time, error) {
	return time.Now(), nil
}

func registerTimeFormats(scr *slang.Script) {
	for _, f := range []struct {
		name, val string
	}{
		{"ANSIC", time.ANSIC},
		{"UnixDate", time.UnixDate},
		{"RubyDate", time.RubyDate},
		{"RFC822", time.RFC822},
		{"RFC822Z", time.RFC822Z},
		{"RFC850", time.RFC850},
		{"RFC1123", time.RFC1123},
		{"RFC1123Z", time.RFC1123Z},
		{"RFC3339", time.RFC3339},
		{"RFC3339Nano", time.RFC3339Nano},
		{"Kitchen", time.Kitchen},
	} {
		scr.RegisterConst("time_"+f.name, f.val)
	}
}

func caveats(rt slang.Runtime, expressions ...string) ([]security.Caveat, error) {
	stmts := make([]caveatflag.Statement, len(expressions))
	for i, expr := range expressions {
		exprAndParam := strings.SplitN(expr, "=", 2)
		if len(exprAndParam) != 2 {
			return nil, fmt.Errorf("incorrect caveat format: %s", expr)
		}
		stmts[i] = caveatflag.Statement{Expr: exprAndParam[0], Params: exprAndParam[1]}
	}
	return caveatflag.Compile(stmts)
}

func allowAllCaveat(rt slang.Runtime) (security.Caveat, error) {
	return security.NewCaveat(security.ConstCaveat, true)
}

func denyAllCaveat(rt slang.Runtime) (security.Caveat, error) {
	return security.NewCaveat(security.ConstCaveat, false)
}

func expiryCaveat(rt slang.Runtime, expiry time.Duration) (security.Caveat, error) {
	return deadlineCaveat(rt, time.Now().Add(expiry))
}

func deadlineCaveat(rt slang.Runtime, deadline time.Time) (security.Caveat, error) {
	c, err := security.NewExpiryCaveat(deadline)
	if err != nil {
		return security.Caveat{}, fmt.Errorf("failed to create expiration caveat: %v", err)
	}
	return c, nil
}

func methodCaveat(rt slang.Runtime, methods ...string) (security.Caveat, error) {
	switch len(methods) {
	case 0:
		return security.Caveat{}, fmt.Errorf("no method arguments")
	case 1:
		return security.NewMethodCaveat(methods[0])
	default:
		return security.NewMethodCaveat(methods[0], methods[1:]...)
	}
}

func thirdPartyCaveatRequirements(rt slang.Runtime, reportServer, reportMethod, reportArguments bool) (security.ThirdPartyRequirements, error) {
	return security.ThirdPartyRequirements{
		ReportServer:    reportServer,
		ReportMethod:    reportMethod,
		ReportArguments: reportArguments,
	}, nil
}

func publicKeyCaveat(rt slang.Runtime, discharger security.PublicKey, location string, requirements security.ThirdPartyRequirements, caveats ...security.Caveat) (security.Caveat, error) {
	switch len(caveats) {
	case 0:
		return security.Caveat{}, fmt.Errorf("no caveat arguments")
	case 1:
		return security.NewPublicKeyCaveat(
			discharger, location, requirements, caveats[0])
	default:
		return security.NewPublicKeyCaveat(
			discharger, location, requirements, caveats[0], caveats[1:]...)
	}
}

func appendCaveats(rt slang.Runtime, a []security.Caveat, b ...security.Caveat) ([]security.Caveat, error) {
	return append(a, b...), nil
}

func init() {
	slang.RegisterFunction(parseTime, "time", `parseTime is the same as time.Parse. The predefined formats from the time package are available as time_<format-name>, e.g. time_ANSIC.`, "layout", "value")
	slang.RegisterFunction(deadline, "time", "deadline is the same as <when>.Add(<expiry)>", "when", "expiry")
	slang.RegisterFunction(localTime, "time", "localtime is the same as time.Now()")

	slang.RegisterFunction(caveats, "caveats", `Create caveats based on the specified caveat expressions which are of the form: "package/path".CaveatName=<caveat-parameters>. For example:

	v.io/v23/security.MethodCaveat={"method"}

	In general, the specific caveat methods (eg. expiryCaveat) should be used.
	`, "caveats")

	slang.RegisterFunction(appendCaveats, "caveats", `appendCaveats is the same as append(a, b...).`, "a", "b")

	slang.RegisterFunction(allowAllCaveat, "caveats", `Create a caveat that allows all requests.`)

	slang.RegisterFunction(denyAllCaveat, "caveats", `Create a caveat that denies all requests.`)

	slang.RegisterFunction(expiryCaveat, "caveats", `Create a Caveat that validates iff the current time is before the current local time and expiry.
	`, "expiry")

	slang.RegisterFunction(deadlineCaveat, "caveats", `Create a Caveat that validates iff the current time is before deadline.`, "deadline")

	slang.RegisterFunction(methodCaveat, "caveats", `Create a caveat that allows the specified methods.`, `methods`)

	slang.RegisterFunction(publicKeyCaveat, "caveats", `Create a public key, ie. third party caveat as per v.io/v23/security.NewPublicKeyCaveat.`, "discharger", "location", "requirements", "caveats")

	slang.RegisterFunction(thirdPartyCaveatRequirements, "caveats", `Create a security.ThirdPartyRequirements instance`, "reportServer", " reportMethod", " reportArguments")

}
