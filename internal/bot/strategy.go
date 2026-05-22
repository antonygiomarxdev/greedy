package bot

import (
	domain "github.com/antonygiomarxdev/greedy/internal/domain/bot"
)

type Action = domain.Action
type Signal = domain.Signal
type BotState = domain.BotState
type Strategy = domain.Strategy
type OrderConfirmer = domain.OrderConfirmer
type OrderFilledListener = domain.OrderFilledListener

const ActionBuy = domain.ActionBuy
const ActionSell = domain.ActionSell
const ActionHold = domain.ActionHold
