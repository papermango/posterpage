# Plan doc

## Abstract

This plan doc is to be maintained by Parth Deshmukh and agents in the creation of a static site generator
that will generate the third iteration of my website. This iteration will be a minimal list-of-links
style website that contains links, a short bio, and a picture of myself. It should be responsive to
different form factors and use as little dependencies as possible, and minimize filesize of the
compiled output.

## Website tech stack

Bootstrap will be used as the main dependency. Dynamic UI frameworks such as React, Angular, and Vue
will intentionally not be used to minimize dependencies. Bootstrap 6 will be used to avoid a jQuery
dependency.

Sass is allowed as a precompilation step.

## SSG design

The SSG itself will be written in Go. Once again, focus should be placed on minimal Go code for
maintainability and usability. Go imports are allowed (excepting cases where entire SSGs are imported.)

## Usage

The SSG should take three configuration files and compile them into a website. The files are as follows:

- site.yml: contains the list of links, the bio, and the names of the favicon and frontmatter pictures.
- head.yml: allows for adding dependencies in the HEAD. this is currently intended for configuring analytics and loading in google fonts
- styles.scss: stylesheet for altering Bootstrap defaults to the desired use. currently, intention is for this scss to be compiled and dropped in as-is to allow maximum flexibility

The pictures also need to be pulled in by the SSG during compilation.

TBD: schema for both YAML files. I heavily recommend making a schema for head.yml to validate and allow only
valid imports and script inclusions. Additionally consider consolidating both YAML files into one if possible.

Decision needed: YAML vs JSON. I think both should be allowed, the choice to make is which one to treat as first-class
and which one to cross-compile into the first-class format.
