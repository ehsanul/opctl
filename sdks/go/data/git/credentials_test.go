package git

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetCredentials(t *testing.T) {
	g := NewGomegaWithT(t)

	ctx := context.Background()
	creds, err := getCredentials(ctx, "https://github.com/Remitly/forge")
	g.Expect(err).To(BeNil())
	g.Expect(creds).To(Equal(""))
}
