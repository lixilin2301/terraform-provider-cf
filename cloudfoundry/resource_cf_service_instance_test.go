package cloudfoundry

import (
	"fmt"
	"testing"

	"code.cloudfoundry.org/cli/cf/errors"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
	"github.com/terraform-providers/terraform-provider-cf/cloudfoundry/cfapi"
)

const serviceInstanceResourceCreate = `

data "cf_org" "org" {
    name = "pcfdev-org"
}
data "cf_space" "space" {
    name = "pcfdev-space"
	org = "${data.cf_org.org.id}"
}
data "cf_service" "mysql" {
    name = "p-mysql"
}

resource "cf_service_instance" "mysql" {
	name = "mysql"
    space = "${data.cf_space.space.id}"
    service_plan = "${data.cf_service.mysql.service_plans["100mb"]}"
	tags = [ "tag-1" , "tag-2" ]
}
`

const serviceInstanceResourceUpdate = `

data "cf_org" "org" {
    name = "pcfdev-org"
}
data "cf_space" "space" {
    name = "pcfdev-space"
	org = "${data.cf_org.org.id}"
}
data "cf_service" "mysql" {
    name = "p-mysql"
}

resource "cf_service_instance" "mysql" {
	name = "mysql-updated"
    space = "${data.cf_space.space.id}"
    service_plan = "${data.cf_service.mysql.service_plans["100mb"]}"
	tags = [ "tag-2", "tag-3", "tag-4" ]
}
`

const serviceInstanceResourceCreateRedis = `

data "cf_org" "org" {
    name = "pcfdev-org"
}
data "cf_space" "space" {
    name = "pcfdev-space"
	org = "${data.cf_org.org.id}"
}
data "cf_service" "redis" {
    name = "p.redis"
}

resource "cf_service_instance" "redis" {
	name = "redis"
    space = "${data.cf_space.space.id}"
    service_plan = "${data.cf_service.redis.service_plans["cache-medium"]}"
	tags = [ "tag-1" , "tag-2" ]
    timeouts {
      create = "30m"
      delete = "30m"
    }
}
`

const serviceInstanceResourceAsyncCreate = `

data "cf_domain" "fake-service-broker-domain" {
    name = "%s"
}

data "cf_org" "org" {
    name = "pcfdev-org"
}
data "cf_space" "space" {
    name = "pcfdev-space"
	org = "${data.cf_org.org.id}"
}
data "cf_service" "fake-service" {
	name = "fake-service"
	depends_on = ["cf_service_broker.fake-service-broker"]
}

resource "cf_route" "fake-service-broker-route" {
	domain = "${data.cf_domain.fake-service-broker-domain.id}"
    space = "${data.cf_space.space.id}"
	hostname = "fake-service-broker"
	depends_on = ["data.cf_domain.fake-service-broker-domain"]
}

resource "cf_app" "fake-service-broker" {
    name = "fake-service-broker"
	url = "file://../vendor/github.com/cf-acceptance-tests/assets/service_broker/"
	space = "${data.cf_space.space.id}"
	timeout = 700

	route {
		default_route = "${cf_route.fake-service-broker-route.id}"
	}

	depends_on = ["cf_route.fake-service-broker-route"]
}

resource "cf_service_broker" "fake-service-broker" {
	name = "fake-service-broker"
	url = "http://fake-service-broker.%s"
	username = "admin"
	password = "admin"
	space = "${data.cf_space.space.id}"
	depends_on = ["cf_app.fake-service-broker"]
}

resource "cf_service_instance" "fake-service" {
	name = "fake-service"
    space = "${data.cf_space.space.id}"
	service_plan = "${cf_service_broker.fake-service-broker.service_plans["fake-service/fake-async-plan"]}"
	depends_on = ["cf_app.fake-service-broker"]
}
`

func TestAccServiceInstance_normal(t *testing.T) {

	ref := "cf_service_instance.mysql"
	resource.Test(t,
		resource.TestCase{
			PreCheck:     func() { testAccPreCheck(t) },
			Providers:    testAccProviders,
			CheckDestroy: testAccCheckServiceInstanceDestroyed([]string{"mysql", "mysql-updated"}, "data.cf_space.space"),
			Steps: []resource.TestStep{

				resource.TestStep{
					Config: serviceInstanceResourceCreate,
					Check: resource.ComposeTestCheckFunc(
						testAccCheckServiceInstanceExists(ref),
						resource.TestCheckResourceAttr(
							ref, "name", "mysql"),
						resource.TestCheckResourceAttr(
							ref, "tags.#", "2"),
						resource.TestCheckResourceAttr(
							ref, "tags.0", "tag-1"),
						resource.TestCheckResourceAttr(
							ref, "tags.1", "tag-2"),
					),
				},

				resource.TestStep{
					Config: serviceInstanceResourceUpdate,
					Check: resource.ComposeTestCheckFunc(
						testAccCheckServiceInstanceExists(ref),
						resource.TestCheckResourceAttr(
							ref, "name", "mysql-updated"),
						resource.TestCheckResourceAttr(
							ref, "tags.#", "3"),
						resource.TestCheckResourceAttr(
							ref, "tags.0", "tag-2"),
						resource.TestCheckResourceAttr(
							ref, "tags.1", "tag-3"),
						resource.TestCheckResourceAttr(
							ref, "tags.2", "tag-4"),
					),
				},
			},
		})
}

func TestAccServiceInstance_async(t *testing.T) {

	ref := "cf_service_instance.redis"

	resource.Test(t,
		resource.TestCase{
			PreCheck:     func() { testAccPreCheck(t) },
			Providers:    testAccProviders,
			CheckDestroy: testAccCheckServiceInstanceDestroyed([]string{"redis"}, "data.cf_space.space"),
			Steps: []resource.TestStep{

				resource.TestStep{
					Config: serviceInstanceResourceCreateRedis,
					Check: resource.ComposeTestCheckFunc(
						testAccCheckServiceInstanceExists(ref),
						resource.TestCheckResourceAttr(
							ref, "name", "redis"),
						resource.TestCheckResourceAttr(
							ref, "tags.#", "2"),
						resource.TestCheckResourceAttr(
							ref, "tags.0", "tag-1"),
						resource.TestCheckResourceAttr(
							ref, "tags.1", "tag-2"),
					),
				},

			},
		})
}

func TestAccServiceBroker_async(t *testing.T) {

	ref := "cf_service_instance.redis"
	refAsync := "cf_service_instance.fake-service"

	resource.Test(t,
		resource.TestCase{
			PreCheck:     func() { testAccPreCheck(t) },
			Providers:    testAccProviders,
			CheckDestroy: testAccCheckServiceInstanceDestroyed([]string{"redis", "fake-service"}, "data.cf_space.space"),
			Steps: []resource.TestStep{

				resource.TestStep{
					Config: fmt.Sprintf(serviceInstanceResourceAsyncCreate, defaultAppDomain(), defaultAppDomain()),
					Check: resource.ComposeTestCheckFunc(
						testAccCheckServiceInstanceExists(refAsync),
						resource.TestCheckResourceAttr(refAsync, "name", "fake-service"),
					),
				},
				resource.TestStep{
					Config: serviceInstanceResourceCreateRedis,
					Check: resource.ComposeTestCheckFunc(
						testAccCheckServiceInstanceExists(ref),
						resource.TestCheckResourceAttr(
							ref, "name", "redis"),
						resource.TestCheckResourceAttr(
							ref, "tags.#", "2"),
						resource.TestCheckResourceAttr(
							ref, "tags.0", "tag-1"),
						resource.TestCheckResourceAttr(
							ref, "tags.1", "tag-2"),
					),
				},

			},
		})
}



func testAccCheckServiceInstanceExists(resource string) resource.TestCheckFunc {

	return func(s *terraform.State) (err error) {

		session := testAccProvider.Meta().(*cfapi.Session)

		rs, ok := s.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("service instance '%s' not found in terraform state", resource)
		}

		session.Log.DebugMessage(
			"terraform state for resource '%s': %# v",
			resource, rs)

		id := rs.Primary.ID

		var (
			serviceInstance cfapi.CCServiceInstance
		)

		sm := session.ServiceManager()
		if serviceInstance, err = sm.ReadServiceInstance(id); err != nil {
			return
		}
		session.Log.DebugMessage(
			"retrieved service instance for resource '%s' with id '%s': %# v",
			resource, id, serviceInstance)

		return
	}
}

func testAccCheckServiceInstanceDestroyed(names []string, spaceResource string) resource.TestCheckFunc {

	return func(s *terraform.State) error {
		session := testAccProvider.Meta().(*cfapi.Session)
		rs, ok := s.RootModule().Resources[spaceResource]
		if !ok {
			return fmt.Errorf("space '%s' not found in terraform state", spaceResource)
		}

		for _, n := range names {
			session.Log.DebugMessage("checking ServiceInstance is Destroyed %s", n)
			if _, err := session.ServiceManager().FindServiceInstance(n, rs.Primary.ID); err != nil {
				switch err.(type) {
				case *errors.ModelNotFoundError:
					return nil
				default:
					continue
				}
			}
			return fmt.Errorf("service instance with name '%s' still exists in cloud foundry", n)
		}
		return nil
	}
}
