// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.lib.discovery;

import com.google.common.base.Equivalence;
import com.google.common.base.Function;
import com.google.common.base.Objects;
import com.google.common.collect.ImmutableMultiset;
import com.google.common.collect.FluentIterable;
import com.google.common.collect.Maps;
import com.google.common.truth.FailureStrategy;
import com.google.common.truth.Subject;
import com.google.common.truth.SubjectFactory;
import com.google.common.truth.Truth;

import java.util.Arrays;
import java.util.List;
import java.util.Map;

import io.v.v23.VFutures;
import io.v.v23.context.VContext;
import io.v.v23.discovery.Advertisement;
import io.v.v23.discovery.Update;
import io.v.v23.verror.VException;

import io.v.x.ref.lib.discovery.AdInfo;

/**
 * Various test utilities for discovery.
 */
public class DiscoveryTestUtil {
    private static final Equivalence<byte[]> BYTE_ARRAY_EQUIVALENCE =
            new Equivalence<byte[]>() {
                @Override
                protected boolean doEquivalent(byte[] a, byte[] b) {
                    return Arrays.equals(a, b);
                }

                @Override
                protected int doHash(byte[] bytes) {
                    return Arrays.hashCode(bytes);
                }
            };

    private static final Equivalence<Advertisement> ADVERTISEMENT_EQUIVALENCE =
            new Equivalence<Advertisement>() {
                @Override
                protected boolean doEquivalent(Advertisement a, Advertisement b) {
                    return a.getId().equals(b.getId())
                            && a.getInterfaceName().equals(b.getInterfaceName())
                            && a.getAddresses().equals(b.getAddresses())
                            && a.getAttributes().equals(b.getAttributes())
                            && Maps.difference(
                                            a.getAttachments(),
                                            b.getAttachments(),
                                            BYTE_ARRAY_EQUIVALENCE)
                                    .areEqual();
                }

                @Override
                protected int doHash(Advertisement ad) {
                    return Objects.hashCode(ad.getId());
                }
            };

    private static final Equivalence<AdInfo> ADINFO_EQUIVALENCE =
            new Equivalence<AdInfo>() {
                @Override
                protected boolean doEquivalent(AdInfo a, AdInfo b) {
                    return ADVERTISEMENT_EQUIVALENCE.equivalent(a.getAd(), b.getAd())
                            && a.getEncryptionAlgorithm().equals(b.getEncryptionAlgorithm())
                            && a.getEncryptionKeys().equals(b.getEncryptionKeys())
                            && a.getHash().equals(b.getHash())
                            && a.getDirAddrs().equals(b.getDirAddrs());
                }

                @Override
                protected int doHash(AdInfo adInfo) {
                    return Objects.hashCode(adInfo.getAd().getId(), adInfo.getHash());
                }
            };

    private static final Function<AdInfo, Equivalence.Wrapper<AdInfo>> ADINFO_WRAPPER =
            new Function<AdInfo, Equivalence.Wrapper<AdInfo>>() {
                @Override
                public Equivalence.Wrapper<AdInfo> apply(AdInfo adinfo) {
                    return ADINFO_EQUIVALENCE.wrap(adinfo);
                }
            };

    /**
     * {@link SubjectFactory} for {@link Advertisement}.
     */
    public static class AdvertisementSubject extends Subject<AdvertisementSubject, Advertisement> {
        public AdvertisementSubject(FailureStrategy fs, Advertisement subject) {
            super(fs, subject);
        }

        public void isEqualTo(Advertisement expected) {
            if (!ADVERTISEMENT_EQUIVALENCE.equivalent(getSubject(), expected)) {
                fail("is equal to", expected);
            }
        }
    }

    private static final SubjectFactory<AdvertisementSubject, Advertisement> ADVERTISEMENT_SF =
            new SubjectFactory<AdvertisementSubject, Advertisement>() {
                @Override
                public AdvertisementSubject getSubject(FailureStrategy fs, Advertisement target) {
                    return new AdvertisementSubject(fs, target);
                }
            };

    public static AdvertisementSubject assertThat(Advertisement actual) {
        return Truth.assertAbout(ADVERTISEMENT_SF).that(actual);
    }

    /**
     * {@link SubjectFactory} for {@link AdInfo}.
     */
    public static class AdInfoSubject extends Subject<AdInfoSubject, AdInfo> {
        public AdInfoSubject(FailureStrategy fs, AdInfo subject) {
            super(fs, subject);
        }

        public void isEqualTo(AdInfo expected) {
            if (!ADINFO_EQUIVALENCE.equivalent(getSubject(), expected)
                    || getSubject().getLost() != expected.getLost()) {
                fail("is equal to", expected);
            }
        }

        public void isEqualTo(AdInfo expected, boolean lost) {
            if (!ADINFO_EQUIVALENCE.equivalent(getSubject(), expected)) {
                fail("is equal to", expected, lost);
            }
            if (getSubject().getLost() != lost) {
                failWithCustomSubject("is equal to", getSubject().getLost(), lost);
            }
        }
    }

    private static final SubjectFactory<AdInfoSubject, AdInfo> ADINFO_SF =
            new SubjectFactory<AdInfoSubject, AdInfo>() {
                @Override
                public AdInfoSubject getSubject(FailureStrategy fs, AdInfo target) {
                    return new AdInfoSubject(fs, target);
                }
            };

    public static AdInfoSubject assertThat(AdInfo actual) {
        return Truth.assertAbout(ADINFO_SF).that(actual);
    }

    /**
     * {@link SubjectFactory} for {@link List<AdInfo>}.
     */
    public static class AdInfoListSubject extends Subject<AdInfoListSubject, List<AdInfo>> {
        public AdInfoListSubject(FailureStrategy fs, List<AdInfo> subject) {
            super(fs, subject);
        }

        public void isEqualTo(AdInfo... expected) {
            List<Equivalence.Wrapper<AdInfo>> actualWrapped =
                    FluentIterable.of(expected).transform(ADINFO_WRAPPER).toList();
            List<Equivalence.Wrapper<AdInfo>> expectedWrapped =
                    FluentIterable.from(getSubject()).transform(ADINFO_WRAPPER).toList();
            if (!ImmutableMultiset.copyOf(actualWrapped)
                    .equals(ImmutableMultiset.copyOf(expectedWrapped))) {
                fail("is equal to", FluentIterable.of(expected));
            }
        }
    }

    private static final SubjectFactory<AdInfoListSubject, List<AdInfo>> ADINFO_LIST_SF =
            new SubjectFactory<AdInfoListSubject, List<AdInfo>>() {
                @Override
                public AdInfoListSubject getSubject(FailureStrategy fs, List<AdInfo> target) {
                    return new AdInfoListSubject(fs, target);
                }
            };

    public static AdInfoListSubject assertThat(List<AdInfo> actual) {
        return Truth.assertAbout(ADINFO_LIST_SF).that(actual);
    }

    /**
     * {@link SubjectFactory} for {@link Update}.
     */
    public static class UpdateSubject extends Subject<UpdateSubject, Update> {
        public UpdateSubject(FailureStrategy fs, Update subject) {
            super(fs, subject);
        }

        public void isEqualTo(VContext ctx, Advertisement expected) throws VException {
            if (!equivalent(ctx, getSubject(), expected)) {
                fail("is equal to", expected);
            }
        }

        private static boolean equivalent(VContext ctx, Update update, Advertisement ad)
                throws VException {
            if (!update.getId().equals(ad.getId())) {
                return false;
            }
            if (!update.getInterfaceName().equals(ad.getInterfaceName())) {
                return false;
            }
            if (!update.getAddresses().equals(ad.getAddresses())) {
                return false;
            }
            for (Map.Entry<String, String> entry : ad.getAttributes().entrySet()) {
                if (!entry.getValue().equals(update.getAttribute(entry.getKey()))) {
                    return false;
                }
            }
            for (Map.Entry<String, byte[]> e : ad.getAttachments().entrySet()) {
                byte[] data = VFutures.sync(update.getAttachment(ctx, e.getKey()));
                if (!Arrays.equals(e.getValue(), data)) {
                    return false;
                }
            }
            return true;
        }
    }

    private static final SubjectFactory<UpdateSubject, Update> UPDATE_SF =
            new SubjectFactory<UpdateSubject, Update>() {
                @Override
                public UpdateSubject getSubject(FailureStrategy fs, Update target) {
                    return new UpdateSubject(fs, target);
                }
            };

    public static UpdateSubject assertThat(Update actual) {
        return Truth.assertAbout(UPDATE_SF).that(actual);
    }
}
