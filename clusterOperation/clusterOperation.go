package clusterOperation

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/digitalocean/godo"
	"github.com/urfave/cli"
	"golang.org/x/oauth2"
)

type TokenSource struct {
	AccessToken string
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

func Create(c *cli.Context) error {
	var newClusterName = []byte(c.String("cluster-name"))
	var contextBucket = []byte("context")

	usr, _ := user.Current()
	if _, err := os.Stat(usr.HomeDir + "/.config/clusterH"); os.IsNotExist(err) {
		os.Mkdir(usr.HomeDir+"/.config/clusterH", 0700)
	}
	db, err := bolt.Open(usr.HomeDir+"/.config/clusterH/clusterH.db", 0644, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if c.String("type") == "do" {
		//  id, _ := uuid.NewV4()
		number := c.Int("number")
		pat := c.String("token")

		tokenSource := &TokenSource{
			AccessToken: pat,
		}
		oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
		client := godo.NewClient(oauthClient)

		var names []string
		names = make([]string, number, number)
		for i := 0; i < number; i++ {
			names[i] = "mcine-" + strconv.Itoa(i)
		}
		createRequest := &godo.DropletMultiCreateRequest{
			Names:  names,
			Region: "nyc3",
			Size:   "512mb",
			SSHKeys: []godo.DropletCreateSSHKey{
				{
					Fingerprint: "0e:4e:20:87:d6:fd:9d:a1:bb:32:33:0c:cd:e3:d0:c7",
				},
			},
			Image: godo.DropletCreateImage{
				Slug: "coreos-stable",
			},
		}

		droplets, _, err := client.Droplets.CreateMultiple(createRequest)

		if err != nil {
			fmt.Printf("Something bad happened: %s\n\n", err)
			return err
		}

		var ipAdresses []string
		ipAdresses = make([]string, number, number)
		for i, droplet := range droplets {
			id := droplet.ID
			droplet, _, _ := client.Droplets.Get(id)
			ip, _ := droplet.PublicIPv4()
			ipAdresses[i] = ip
		}

		fmt.Printf("Cluster created. IP adresses of cluster's machines: %v", ipAdresses)

		// store ip addresses of cluster's members
		err = db.Update(func(tx *bolt.Tx) error {
			bucket, _ := tx.CreateBucketIfNotExists(newClusterName)

			key := []byte("members")
			stringByte := "\x00" + strings.Join(ipAdresses, "\x20\x00")
			value := []byte(stringByte)

			err = bucket.Put(key, value)
			if err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			log.Fatal(err)
		}

		// store ip addresses of cluster's members
		err = db.Update(func(tx *bolt.Tx) error {
			bucket, _ := tx.CreateBucketIfNotExists(contextBucket)

			key := []byte("currentContext")
			value := []byte(newClusterName)

			err = bucket.Put(key, value)
			if err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			log.Fatal(err)
		}

		//retrieve current context
		err = db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(contextBucket)
			if bucket == nil {
				return fmt.Errorf("Bucket %q not found!", contextBucket)
			}

			key := []byte("currentContext")

			val := bucket.Get(key)
			fmt.Println(string(val))

			return nil
		})

		if err != nil {
			log.Fatal(err)
		}
	}
	return nil
}
