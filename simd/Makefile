CC       = cc
CFLAGS   = -O3
LDFLAGS  = -lm

UNAME_M = $(shell uname -m)

ifeq ($(UNAME_M),arm64)
	CFLAGS += -march=armv8-a+simd
endif
ifeq ($(UNAME_M),aarch64)
	CFLAGS += -march=armv8-a+simd
endif
ifeq ($(UNAME_M),x86_64)
	CFLAGS += -mavx2 -mfma
endif

DIR      = $(dir $(lastword $(MAKEFILE_LIST)))

SRCS     = $(filter-out test_%.c, $(notdir $(wildcard $(DIR)*.c)))
OBJS     = $(addprefix $(DIR), $(SRCS:.c=.o))
TESTS    = $(patsubst $(DIR)%.c, %, $(wildcard $(DIR)test_*.c))

.PHONY: all test clean

all: $(TESTS)

$(DIR)%.o: $(DIR)%.c $(DIR)distance.h
	$(CC) -c $(CFLAGS) $< -o $@

$(foreach t,$(TESTS),$(eval $(t): $(DIR)$(t).c $(OBJS)))
$(foreach t,$(TESTS),$(eval \
	$(t): ; $(CC) $(CFLAGS) $(DIR)$(t).c $(OBJS) $(LDFLAGS) -o $(DIR)$(t) \
))

test: $(TESTS)
	@for t in $(TESTS); do echo "--- $$t ---"; $(DIR)$$t || exit 1; done

clean:
	rm -f $(OBJS) $(addprefix $(DIR), $(TESTS))
