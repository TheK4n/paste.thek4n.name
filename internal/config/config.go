package config

import (
	"time"
)

// Body size
const ONE_MEBIBYTE = 1048576
const UNPREVELEGED_MAX_BODY_SIZE = ONE_MEBIBYTE
const PREVELEGED_MAX_BODY_SIZE = ONE_MEBIBYTE * 100

// TTL
const SECONDS_IN_MONTH = 60 * 60 * 24 * 30
const DEFAULT_TTL_SECONDS = time.Second * SECONDS_IN_MONTH

const MIN_TTL = time.Second * 0

const SECONDS_IN_YEAR = 60 * 60 * 24 * 30 * 12
const MAX_TTL = time.Second * SECONDS_IN_YEAR

// Key length
const MAX_KEY_LENGTH = 20
const DEFAULT_KEY_LENGTH = 14
const UNPRIVELEGED_MIN_KEY_LENGTH = 14
const PRIVELEGED_MIN_KEY_LENGTH = 3

const HEALTHCHECK_TIMEOUT = time.Second * 3

// Key generation config
const ATTEMPTS_TO_INCREASE_KEY_MIN_LENGHT = 20
const CHARSET = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Quota config
const QUOTA_RESET_PERIOD = 24 * time.Hour
const QUOTA = 50

// Compress
const COMPRESS_THRESHOLD_BYTES = 4096

const APIKEYUSAGE_EXCHANGE = "apikeysusage"
