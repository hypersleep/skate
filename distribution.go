package main

import(
	"log"
	"time"
	"errors"
	"net/url"
	"net/http"
	"io/ioutil"
	"encoding/json"
)

type(
	CreatedAt struct {
		Created time.Time `json:"created"`
	}

	Config struct {
		Digest string `json:"digest"`
	}

	Digest struct {
		Config *Config `json:"config"`
	}

	Tags struct {
		Tags []string `json:"tags"`
	}

	Distribution struct {
		Repositories []string `json:"repositories"`
		Manifests    []string `json:"manifests"`
		url          *url.URL
		ttl          time.Duration
		except       []string
	}
)

func (distribution *Distribution) exceptFilter(repository string) bool {
	for _, exceptValue := range distribution.except {
		if exceptValue == repository {
			return false
		}
	}
	return true
}

func (distribution *Distribution) getRepositories() error {
	resp, err := http.Get(distribution.url.String() + "/_catalog?n=99999")
	if err != nil {
		return err
	}

	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, &distribution)
	if err != nil {
		return err
	}

	// filtering by except slice
	repositories := distribution.Repositories[:0]

	for _, repositoryValue := range distribution.Repositories {
		if distribution.exceptFilter(repositoryValue) {
			repositories = append(repositories,repositoryValue)
		}
	}

	distribution.Repositories = repositories

	return nil
}

func (distribution *Distribution) getTags(repository string) (*Tags, error) {
	tags := &Tags{}

	resp, err := http.Get(distribution.url.String() + repository + "/tags/list")
	if err != nil {
		return tags, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return tags, err
	}

	err = json.Unmarshal(b, &tags)
	if err != nil {
		return tags, err
	}

	return tags, nil
}

func (distribution *Distribution) getTagCreatedAt(repository string, tag string) (time.Time, string, error) {
	var createdAt time.Time
	var dockerContentDigest string
	var acceptHeader string = "application/vnd.docker.distribution.manifest.v2+json"

	digest := &Digest{&Config{}}

	created := &CreatedAt{}

	client := &http.Client{}
	req, err := http.NewRequest("GET", distribution.url.String() + repository + "/manifests/" + tag, nil)
	if err != nil {
		return createdAt, dockerContentDigest, err
	}

	req.Header.Add("Accept", acceptHeader)
	resp, err := client.Do(req)
	if err != nil {
		return createdAt, dockerContentDigest, err
	}

	dockerContentDigest = resp.Header.Get("Docker-Content-Digest")

	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return createdAt, dockerContentDigest, err
	}

	err = json.Unmarshal(b, &digest)
	if err != nil {
		return createdAt, dockerContentDigest, err
	}

	client = &http.Client{}
	req, err = http.NewRequest("GET", distribution.url.String() + repository + "/blobs/" + digest.Config.Digest, nil)
	if err != nil {
		return createdAt, dockerContentDigest, err
	}

	req.Header.Add("Accept", acceptHeader)
	resp, err = client.Do(req)
	if err != nil {
		return createdAt, dockerContentDigest, err
	}

	b, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return createdAt, dockerContentDigest, err
	}

	err = json.Unmarshal(b, &created)
	if err != nil {
		return createdAt, dockerContentDigest, err
	}

	createdAt = created.Created

	return createdAt, dockerContentDigest, nil
}

func (distribution *Distribution) removeTag(repository string, digest string) error {
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", distribution.url.String() + repository + "/manifests/" + digest, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusAccepted {
		return errors.New("non 200 delete status")
	}

	return nil
}

func (distribution *Distribution) check() error {
	distribution.url.Path = distribution.url.Path + "/v2/"

	resp, err := http.Get(distribution.url.String())
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New("/v2 endpoint returned non 200 status")
	}

	return nil
}

func (distribution *Distribution) cleanup() error {
	err := distribution.getRepositories()
	if err != nil {
		return err
	}

	for _, repository := range distribution.Repositories {
		tags, err := distribution.getTags(repository)
		if err != nil {
			return err
		}

		for _, tag := range tags.Tags {
			log.Println("Removing:", repository, tag)
			createdAt, digest, err := distribution.getTagCreatedAt(repository, tag)
			if err != nil {
				log.Println("repository:", repository, "tag:", tag, "got an error:", err)
			} else {
				if time.Since(createdAt) > distribution.ttl {
					err = distribution.removeTag(repository, digest)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
