package git

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opctl/opctl/sdks/go/model"
)

var _ = Context("_git", func() {
	Context("TryResolve", func() {
		Context("localFSProvider.TryResolve errors", func() {
			It("should return err", func() {
				/* arrange */
				dataDir, err := ioutil.TempDir("", "")
				if err != nil {
					panic(err)
				}
				objectUnderTest := New(dataDir)

				/* act */
				_, actualError := objectUnderTest.TryResolve(
					context.Background(),
					make(chan model.Event),
					"callID",
					"/not/exists",
				)

				/* assert */
				Expect(actualError).To(MatchError("invalid git ref: missing version"))
			})
		})
		Context("localFSProvider.TryResolve doesn't err", func() {
			Context("localFSProvider.TryResolve returns handle", func() {
				It("should return handle", func() {
					wd, err := os.Getwd()
					if err != nil {
						panic(err)
					}
					opRef := filepath.Join(wd, "../testdata/testop")

					objectUnderTest := New(filepath.Dir(opRef))

					/* act */
					actualHandle, actualErr := objectUnderTest.TryResolve(
						context.Background(),
						make(chan model.Event),
						"callID",
						opRef,
					)

					/* assert */
					Expect(actualErr).To(BeNil())
					Expect(actualHandle.Ref()).To(Equal(opRef))
				})
			})
			Context("FSProvider.TryResolve doesn't return a handle", func() {
				Context("puller.Pull errors", func() {
					It("should return err", func() {
						dataDir, err := ioutil.TempDir("", "")
						if err != nil {
							panic(err)
						}
						objectUnderTest := New(dataDir)

						/* act */
						_, actualErr := objectUnderTest.TryResolve(
							context.Background(),
							make(chan model.Event),
							"callID",
							"/not/exists",
						)

						/* assert */
						Expect(actualErr).To(MatchError("invalid git ref: missing version"))
					})
				})
				Context("puller.Pull doesn't error", func() {
					It("should return expected result", func() {
						/* arrange */
						// some public repo that's relatively small
						providedRef := "github.com/opspec-pkgs/_.op.create#3.3.1"
						basePath, err := ioutil.TempDir("", "")
						if err != nil {
							panic(err)
						}
						objectUnderTest := New(basePath)
						expectedHandle := newHandle(filepath.Join(basePath, providedRef), providedRef)

						/* act */
						actualHandle, actualError := objectUnderTest.TryResolve(
							context.Background(),
							make(chan model.Event),
							"callID",
							providedRef,
						)

						/* assert */
						Expect(actualHandle).To(Equal(expectedHandle))
						Expect(actualError).To(BeNil())
					})
				})
			})
		})
		Context("called in parallel w/ same pkg ref", func() {
			It("should return expected result", func() {
				/* arrange */
				// some public repo that's relatively small
				providedRef := "github.com/opspec-pkgs/_.op.create#3.3.1"

				basePath, err := ioutil.TempDir("", "")
				if err != nil {
					panic(err)
				}

				objectUnderTest := New(basePath)

				expectedResult := newHandle(filepath.Join(basePath, providedRef), providedRef)

				var (
					actualResult1,
					actualResult2 model.DataHandle
				)
				var (
					actualErr1,
					actualErr2 error
				)

				/* act */
				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					actualResult1, actualErr1 = objectUnderTest.TryResolve(
						context.Background(),
						make(chan model.Event),
						"callID",
						providedRef,
					)
					wg.Done()
				}()

				wg.Add(1)
				go func() {
					actualResult2, actualErr2 = objectUnderTest.TryResolve(
						context.Background(),
						make(chan model.Event),
						"callID",
						providedRef,
					)
					wg.Done()
				}()

				wg.Wait()

				/* assert */
				Expect(actualErr1).To(BeNil())
				Expect(actualErr2).To(BeNil())
				Expect(actualResult1.Path()).To(Equal(expectedResult.Path()))
				Expect(actualResult2.Path()).To(Equal(expectedResult.Path()))
			})
		})
		Context("called in parallel w/ different pkg ref", func() {
			It("should return expected result", func() {
				/* arrange */
				// some public repo that's relatively small
				providedRef1 := "github.com/opspec-pkgs/_.op.create#3.3.1"
				providedRef2 := "github.com/opspec-pkgs/_.op.create#3.0.0"

				basePath, err := ioutil.TempDir("", "")
				if err != nil {
					panic(err)
				}

				objectUnderTest := New(basePath)

				expectedResult1 := newHandle(filepath.Join(basePath, providedRef1), providedRef1)
				expectedResult2 := newHandle(filepath.Join(basePath, providedRef2), providedRef2)

				var (
					actualResult1,
					actualResult2 model.DataHandle
				)
				var (
					actualErr1,
					actualErr2 error
				)

				/* act */
				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					actualResult1, actualErr1 = objectUnderTest.TryResolve(
						context.Background(),
						make(chan model.Event),
						"callID",
						providedRef1,
					)
					wg.Done()
				}()

				wg.Add(1)
				go func() {
					actualResult2, actualErr2 = objectUnderTest.TryResolve(
						context.Background(),
						make(chan model.Event),
						"callID",
						providedRef2,
					)
					wg.Done()
				}()

				wg.Wait()

				/* assert */
				Expect(actualErr1).To(BeNil())
				Expect(actualResult1.Path()).To(Equal(expectedResult1.Path()))

				Expect(actualErr2).To(BeNil())
				Expect(actualResult2.Path()).To(Equal(expectedResult2.Path()))
			})
		})
	})
})
