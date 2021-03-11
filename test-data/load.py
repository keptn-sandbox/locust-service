from locust import HttpUser, between, task


class WebsiteUser(HttpUser):
    wait_time = between(5, 15)
    
    @task
    def index(self):
        self.client.get("/")

    @task
    def get_cart(self):
        self.client.get("/carts/1")

    @task
    def get_cart_items(self):
        self.client.get("/carts/1/items")

    @task
    def health(self):
        self.client.get("/health")

