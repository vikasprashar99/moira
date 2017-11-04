package handler

import (
	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/moira-alert/moira/api"
	"github.com/moira-alert/moira/api/controller"
	"github.com/moira-alert/moira/api/dto"
	"github.com/moira-alert/moira/api/middleware"
	"net/http"
)

func subscription(router chi.Router) {
	router.Get("/", getUserSubscriptions)
	router.Put("/", createSubscription)
	router.Route("/{subscriptionId}", func(router chi.Router) {
		router.Use(middleware.SubscriptionContext)
		router.Put("/", updateSubscription)
		router.Delete("/", removeSubscription)
		router.Put("/test", sendTestNotification)
	})
}

func getUserSubscriptions(writer http.ResponseWriter, request *http.Request) {
	userLogin := middleware.GetLogin(request)
	contacts, err := controller.GetUserSubscriptions(database, userLogin)
	if err != nil {
		render.Render(writer, request, err)
		return
	}
	if err := render.Render(writer, request, contacts); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func createSubscription(writer http.ResponseWriter, request *http.Request) {
	subscription := &dto.Subscription{}
	if err := render.Bind(request, subscription); err != nil {
		render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}
	userLogin := middleware.GetLogin(request)

	if err := controller.CreateSubscription(database, userLogin, subscription); err != nil {
		render.Render(writer, request, err)
		return
	}
	if err := render.Render(writer, request, subscription); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func updateSubscription(writer http.ResponseWriter, request *http.Request) {
	subscription := &dto.Subscription{}
	if err := render.Bind(request, subscription); err != nil {
		render.Render(writer, request, api.ErrorInvalidRequest(err))
		return
	}
	userLogin := middleware.GetLogin(request)
	subscriptionID := middleware.GetSubscriptionID(request)
	err := controller.CheckUserPermissionsForSubscription(database, subscriptionID, userLogin)
	if err != nil {
		render.Render(writer, request, err)
	}

	if err := controller.UpdateSubscription(database, subscriptionID, userLogin, subscription); err != nil {
		render.Render(writer, request, err)
		return
	}
	if err := render.Render(writer, request, subscription); err != nil {
		render.Render(writer, request, api.ErrorRender(err))
		return
	}
}

func removeSubscription(writer http.ResponseWriter, request *http.Request) {
	userLogin := middleware.GetLogin(request)
	subscriptionID := middleware.GetSubscriptionID(request)
	err := controller.CheckUserPermissionsForSubscription(database, subscriptionID, userLogin)
	if err != nil {
		render.Render(writer, request, err)
	}

	if err := controller.RemoveSubscription(database, subscriptionID); err != nil {
		render.Render(writer, request, err)
	}
}

func sendTestNotification(writer http.ResponseWriter, request *http.Request) {
	userLogin := middleware.GetLogin(request)
	subscriptionID := middleware.GetSubscriptionID(request)
	err := controller.CheckUserPermissionsForSubscription(database, subscriptionID, userLogin)
	if err != nil {
		render.Render(writer, request, err)
	}

	if err := controller.SendTestNotification(database, subscriptionID); err != nil {
		render.Render(writer, request, err)
	}
}
