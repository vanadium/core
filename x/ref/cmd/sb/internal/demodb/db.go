// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package demodb

import (
	"fmt"
	"time"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/vdl"
	"v.io/v23/vom"
)

type kv struct {
	key   string
	value *vom.RawBytes
}

type collection struct {
	name string
	rows []kv
}

var demoCollections []collection

func init() {
	// We can't call vom.RawBytesOf directly in the demoCollections var
	// initializer, because of init ordering issues with vdl.Register{,Native}
	// calls in the vdl-generated files.
	demoCollections = []collection{
		collection{
			name: "Customers",
			rows: []kv{
				kv{
					"001",
					vom.RawBytesOf(Customer{"John Smith", 1, true, AddressInfo{"1 Main St.", "Palo Alto", "CA", "94303"}, CreditReport{Agency: CreditAgencyEquifax, Report: AgencyReportEquifaxReport{EquifaxCreditReport{'A'}}}}),
				},
				kv{
					"001001",
					vom.RawBytesOf(Invoice{1, 1000, 42, AddressInfo{"1 Main St.", "Palo Alto", "CA", "94303"}}),
				},
				kv{
					"001002",
					vom.RawBytesOf(Invoice{1, 1003, 7, AddressInfo{"2 Main St.", "Palo Alto", "CA", "94303"}}),
				},
				kv{
					"001003",
					vom.RawBytesOf(Invoice{1, 1005, 88, AddressInfo{"3 Main St.", "Palo Alto", "CA", "94303"}}),
				},
				kv{
					"002",
					vom.RawBytesOf(Customer{"Bat Masterson", 2, true, AddressInfo{"777 Any St.", "Collins", "IA", "50055"}, CreditReport{Agency: CreditAgencyTransUnion, Report: AgencyReportTransUnionReport{TransUnionCreditReport{80}}}}),
				},
				kv{
					"002001",
					vom.RawBytesOf(Invoice{2, 1001, 166, AddressInfo{"777 Any St.", "Collins", "IA", "50055"}}),
				},
				kv{
					"002002",
					vom.RawBytesOf(Invoice{2, 1002, 243, AddressInfo{"888 Any St.", "Collins", "IA", "50055"}}),
				},
				kv{
					"002003",
					vom.RawBytesOf(Invoice{2, 1004, 787, AddressInfo{"999 Any St.", "Collins", "IA", "50055"}}),
				},
				kv{
					"002004",
					vom.RawBytesOf(Invoice{2, 1006, 88, AddressInfo{"101010 Any St.", "Collins", "IA", "50055"}}),
				},
			},
		},
		collection{
			name: "Numbers",
			rows: []kv{
				kv{
					"001",
					vom.RawBytesOf(Numbers{byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), int64(128), float32(3.14159), float64(2.71828182846)}),
				},
				kv{
					"002",
					vom.RawBytesOf(Numbers{byte(9), uint16(99), uint32(999), uint64(9999999), int16(9), int32(99), int64(88), float32(1.41421356237), float64(1.73205080757)}),
				},
				kv{
					"003",
					vom.RawBytesOf(Numbers{byte(210), uint16(210), uint32(210), uint64(210), int16(210), int32(210), int64(210), float32(210.0), float64(210.0)}),
				},
			},
		},
		collection{
			name: "Composites",
			rows: []kv{
				kv{
					"uno",
					vom.RawBytesOf(Composite{Array2String{"foo", "bar"}, []int32{1, 2}, map[int32]struct{}{1: struct{}{}, 2: struct{}{}}, map[string]int32{"foo": 1, "bar": 2}}),
				},
			},
		},
		collection{
			name: "Recursives",
			rows: []kv{
				kv{
					"alpha",
					vom.RawBytesOf(Recursive{nil, &Times{time.Unix(123456789, 42244224), time.Duration(1337)}, map[Array2String]Recursive{
						Array2String{"a", "b"}: Recursive{},
						Array2String{"x", "y"}: Recursive{vom.RawBytesOf(CreditReport{Agency: CreditAgencyExperian, Report: AgencyReportExperianReport{ExperianCreditReport{ExperianRatingGood}}}), nil, map[Array2String]Recursive{
							Array2String{"alpha", "beta"}: Recursive{vom.RawBytesOf(FooType{Bar: BarType{Baz: BazType{Name: "hello", TitleOrValue: TitleOrValueTypeValue{Value: 42}}}}), nil, nil},
						}},
						Array2String{"u", "v"}: Recursive{vom.RawBytesOf(vdl.TypeOf(Recursive{})), nil, nil},
					}}),
				},
			},
		},
		collection{
			name: "Students",
			rows: []kv{
				kv{
					"1",
					vom.RawBytesOf(Student{Name: "John Smith", TestTime: t("Jul 22 12:34:56 PDT 2015"), Score: ActOrSatScoreActScore{Value: 36}}),
				},
				kv{
					"2",
					vom.RawBytesOf(Student{Name: "Mary Jones", TestTime: t("Sep 4 01:23:45 PDT 2015"), Score: ActOrSatScoreSatScore{Value: 1200}}),
				},
			},
		},
		collection{
			name: "AnythingGoes",
			rows: []kv{
				kv{
					"bar",
					vom.RawBytesOf(AnythingGoes{NameOfType: "Student", Anything: vom.RawBytesOf(Student{Name: "John Smith", Score: ActOrSatScoreActScore{Value: 36}})}),
				},
				kv{
					"baz",
					vom.RawBytesOf(AnythingGoes{NameOfType: "Customer", Anything: vom.RawBytesOf(Customer{"Bat Masterson", 2, true, AddressInfo{"777 Any St.", "Collins", "IA", "50055"}, CreditReport{Agency: CreditAgencyTransUnion, Report: AgencyReportTransUnionReport{TransUnionCreditReport{80}}}})}),
				},
			},
		},
	}
}

func t(timeStr string) time.Time {
	t, _ := time.Parse("Jan 2 15:04:05 MST 2006", timeStr)
	return t
}

// Creates demo collections in the provided database. Collections are destroyed
// and recreated if they already exist.
func PopulateDemoDB(ctx *context.T, db syncbase.Database) error {
	for i, c := range demoCollections {
		if err := syncbase.RunInBatch(ctx, db, wire.BatchOptions{}, func(db syncbase.BatchDatabase) error {
			dc := db.Collection(ctx, c.name)
			if err := dc.Destroy(ctx); err != nil {
				return fmt.Errorf("Destroy %v failed: %v", dc.Id(), err)
			}
			if err := dc.Create(ctx, nil); err != nil {
				return fmt.Errorf("Create %v failed: %v", dc.Id(), err)
			}
			for _, kv := range c.rows {
				if err := dc.Put(ctx, kv.key, kv.value); err != nil {
					return fmt.Errorf("Put %q into %v failed: %v", kv.key, dc.Id(), err)
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed creating collection %s (%d/%d): %v", c.name, i+1, len(demoCollections), err)
		}
	}
	return nil
}
