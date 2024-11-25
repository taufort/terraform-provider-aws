// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package bedrock_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	"github.com/aws/aws-sdk-go-v2/service/bedrock/types"
	sdkacctest "github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	tfbedrock "github.com/hashicorp/terraform-provider-aws/internal/service/bedrock"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// Regions are hard coded due to limited availability of Bedrock service
const (
	foundationModelARN  = "arn:aws:bedrock:eu-central-1::foundation-model/anthropic.claude-3-5-sonnet-20240620-v1:0"                                               // lintignore:AWSAT003,AWSAT005
	inferenceProfileARN = "arn:aws:bedrock:us-west-2:${data.aws_caller_identity.current.account_id}:inference-profile/us.anthropic.claude-3-5-haiku-20241022-v1:0" // lintignore:AWSAT003,AWSAT005
)

func TestAccBedrockInferenceProfile_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var inferenceprofile bedrock.GetInferenceProfileOutput

	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_bedrock_inference_profile.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			acctest.PreCheckPartitionHasService(t, names.BedrockEndpointID)
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.BedrockServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckInferenceProfileDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccInferenceProfileConfig_basic(rName, foundationModelARN, "t1", "v1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckInferenceProfileExists(ctx, resourceName, &inferenceprofile),
					resource.TestCheckResourceAttr(resourceName, names.AttrDescription, "Test description"),
					resource.TestCheckResourceAttrSet(resourceName, "updated_at"),
					resource.TestCheckResourceAttrSet(resourceName, names.AttrCreatedAt),
					resource.TestCheckResourceAttr(resourceName, "models.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "models.0.model_arn", foundationModelARN),
					acctest.MatchResourceAttrRegionalARN(resourceName, names.AttrARN, "bedrock", regexache.MustCompile(`application-inference-profile/.+`)),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"model_source.#",
					"model_source.0.%",
					"model_source.0.copy_from",
				},
			},
		},
	})
}

func TestAccBedrockInferenceProfile_tags(t *testing.T) {
	ctx := acctest.Context(t)
	var before, after bedrock.GetInferenceProfileOutput

	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_bedrock_inference_profile.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			acctest.PreCheckPartitionHasService(t, names.BedrockEndpointID)
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.BedrockServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckInferenceProfileDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccInferenceProfileConfig_basic(rName, foundationModelARN, "t1", "v1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckInferenceProfileExists(ctx, resourceName, &before),
				),
			},
			{
				// tag only change
				Config: testAccInferenceProfileConfig_basic(rName, foundationModelARN, "t1", "v2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckInferenceProfileExists(ctx, resourceName, &after),
					testAccCheckInferenceProfileNotRecreated(&before, &after),
				),
			},
		},
	})
}

func TestAccBedrockInferenceProfile_disappears(t *testing.T) {
	ctx := acctest.Context(t)

	var inferenceprofile bedrock.GetInferenceProfileOutput
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_bedrock_inference_profile.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			acctest.PreCheckPartitionHasService(t, names.BedrockEndpointID)
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.BedrockServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckInferenceProfileDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccInferenceProfileConfig_basic(rName, inferenceProfileARN, "t1", "v1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckInferenceProfileExists(ctx, resourceName, &inferenceprofile),
					acctest.CheckFrameworkResourceDisappears(ctx, acctest.Provider, tfbedrock.ResourceInferenceProfile, resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckInferenceProfileDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.Provider.Meta().(*conns.AWSClient).BedrockClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "aws_bedrock_inference_profile" {
				continue
			}

			input := &bedrock.GetInferenceProfileInput{
				InferenceProfileIdentifier: aws.String(rs.Primary.ID),
			}

			_, err := conn.GetInferenceProfile(ctx, input)

			if errs.IsA[*types.ResourceNotFoundException](err) {
				return nil
			}

			if err != nil {
				return create.Error(names.Bedrock, create.ErrActionCheckingDestroyed, tfbedrock.ResNameInferenceProfile, rs.Primary.ID, err)
			}

			return create.Error(names.Bedrock, create.ErrActionCheckingDestroyed, tfbedrock.ResNameInferenceProfile, rs.Primary.ID, errors.New("not destroyed"))
		}

		return nil
	}
}

func testAccCheckInferenceProfileExists(ctx context.Context, name string, inferenceprofile *bedrock.GetInferenceProfileOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return create.Error(names.Bedrock, create.ErrActionCheckingExistence, tfbedrock.ResNameInferenceProfile, name, errors.New("not found"))
		}

		if rs.Primary.ID == "" {
			return create.Error(names.Bedrock, create.ErrActionCheckingExistence, tfbedrock.ResNameInferenceProfile, name, errors.New("not set"))
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).BedrockClient(ctx)

		resp, err := conn.GetInferenceProfile(ctx, &bedrock.GetInferenceProfileInput{
			InferenceProfileIdentifier: aws.String(rs.Primary.ID),
		})
		if err != nil {
			return create.Error(names.Bedrock, create.ErrActionCheckingExistence, tfbedrock.ResNameInferenceProfile, rs.Primary.ID, err)
		}

		*inferenceprofile = *resp

		return nil
	}
}

func testAccPreCheck(ctx context.Context, t *testing.T) {
	conn := acctest.Provider.Meta().(*conns.AWSClient).BedrockClient(ctx)

	input := &bedrock.ListInferenceProfilesInput{}

	_, err := conn.ListInferenceProfiles(ctx, input)

	if acctest.PreCheckSkipError(err) {
		t.Skipf("skipping acceptance testing: %s", err)
	}
	if err != nil {
		t.Fatalf("unexpected PreCheck error: %s", err)
	}
}

func testAccCheckInferenceProfileNotRecreated(before, after *bedrock.GetInferenceProfileOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if before, after := aws.ToString(before.InferenceProfileId), aws.ToString(after.InferenceProfileId); before != after {
			return create.Error(names.Bedrock, create.ErrActionCheckingNotRecreated, tfbedrock.ResNameInferenceProfile, before, errors.New("recreated"))
		}

		return nil
	}
}

func testAccInferenceProfileConfig_basic(rName, source, tagKey, tagVal string) string {
	return fmt.Sprintf(`
data "aws_caller_identity" "current" {}

resource "aws_bedrock_inference_profile" "test" {
  name        = %[1]q
  description = "Test description"

  model_source {
    copy_from = %[2]q
  }

  tags = {
    Name  = "test"
    %[3]q = %[4]q
  }
}
`, rName, source, tagKey, tagVal)
}
