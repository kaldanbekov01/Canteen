package main

import (
	"fmt"
	"go-auth/models"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/sessions"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var store = sessions.NewCookieStore([]byte("your-secret-key"))

func main() {
	dsn := "root:@tcp(127.0.0.1:3306)/canteen?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	runMigrations(db)

	http.HandleFunc("/", mainPageHandler)
	http.HandleFunc("/login", loginPageHandler)
	http.HandleFunc("/addToBasket", withAuth(addToBasket))
	http.HandleFunc("/deleteFromBasket", deleteFromBasket)
	http.HandleFunc("/orders", order)
	http.HandleFunc("/confirmOrder", confirmOrder) 
	http.HandleFunc("/admin", admin) 



	fmt.Printf("Starting server for testing...\n")
	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatal(err)
	}
}

func withAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "session-name")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		userID, ok := session.Values["userID"]
		if !ok || userID == nil {
			// If userID is not found, redirect to login page
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		handler(w, r)
	}
}

func addToBasket(w http.ResponseWriter, r *http.Request) {
	dsn := "root:@tcp(127.0.0.1:3306)/canteen?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session, err := store.Get(r, "session-name")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	userID, ok := session.Values["userID"].(int)
	if !ok {
		log.Printf("UserID not found in session or not of type int: %v", session.Values["userID"])
		http.Error(w, "User ID not found in session or not of type int", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodPost {
		itemID := r.FormValue("itemID")
		valueStr := r.FormValue("value")

		value, err := strconv.Atoi(valueStr)
		if err != nil {
			http.Error(w, "Invalid value", http.StatusBadRequest)
			return
		}

		var user models.User
		result1 := db.First(&user, userID)
		if result1.Error != nil {
			http.Error(w, "Failed to get user from the database", http.StatusInternalServerError)
			return
		}

		var item models.Item
		result2 := db.First(&item, itemID)
		if result2.Error != nil {
			http.Error(w, "Failed to get item from the database", http.StatusInternalServerError)
			return
		}

		var basket models.Basket

		result := db.Where("item_id = ?", itemID).First(&basket)
		if result.Error != nil {
			basket = models.Basket{
				ItemID:   item.ID,
				UserID:   uint(userID),
				Quantity: value,
			}
			if err := db.Create(&basket).Error; err != nil {
				http.Error(w, "Failed to create basket entry", http.StatusInternalServerError)
				return
			}
		} else {
			// Update the existing basket entry's quantity
			basket.Quantity += value
			if err := db.Save(&basket).Error; err != nil {
				http.Error(w, "Failed to update basket quantity", http.StatusInternalServerError)
				return
			}
		}

		if item.Count > 0 {
			item.Count -= value
			result := db.Save(&item)
			if result.Error != nil {
				http.Error(w, "Failed to update item count", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "Item count is already zero", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Item count updated successfully"))
		return
	}
	var basketItems []models.Basket
	db.Where("user_id = ? AND deleted_at IS NULL", userID).Find(&basketItems)

	var basketItemsWithDetails []map[string]interface{}

	for _, basketItem := range basketItems {
		var item models.Item
		result := db.First(&item, basketItem.ItemID)
		if result.Error != nil {
			http.Error(w, "Failed to get item details from the database", http.StatusInternalServerError)
			return
		}
		basketItemsWithDetails = append(basketItemsWithDetails, map[string]interface{}{
			"Basket": basketItem,
			"Item":   item,
		})
	}

	categories, err := getCategoryAndItemData(db)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var user models.User
	db.First(&user, userID)

	data := map[string]interface{}{
		"Username":   user.Name,
		"Categories": categories,
		"Items":      basketItemsWithDetails,
	}

	tmpl, err := template.ParseFiles("html/basket.html")
	if err != nil {
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		return
	}
}

func deleteFromBasket(w http.ResponseWriter, r *http.Request) {
	dsn := "root:@tcp(127.0.0.1:3306)/canteen?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodPost {
		itemID := r.FormValue("itemID")
		var basket models.Basket

		result := db.Where("item_id = ?", itemID).First(&basket)
		if result.Error != nil {
			http.Error(w, result.Error.Error(), http.StatusNotFound)
			return
		}

		err := db.Delete(&basket).Error
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Item deleted successfully"))
	}
}

func mainPageHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		http.ServeFile(w, r, "html/index.html")
	case "POST":
		abcPostHandler(w, r)
	default:
		fmt.Fprintf(w, "Only GET and POST methods are allowed")
	}
}

func loginPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		http.ServeFile(w, r, "html/forms.html")
	} else {
		fmt.Fprintf(w, "Only GET method is allowed for login page")
	}
}

func abcPostHandler(w http.ResponseWriter, r *http.Request) {
	dsn := "root:@tcp(127.0.0.1:3306)/canteen?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, "ParseForm() err : %v", err)
		return
	}
	categories, err := getCategoryAndItemData(db)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var user models.User

	email := r.FormValue("email")
	password := r.FormValue("password")


	if err := db.Where("email = ?", email).First(&user).Error; err != nil {
		fmt.Fprintf(w, "Email not found\n")
		return
	}

	if user.Password != password {
		fmt.Fprintf(w, "Incorrect password\n")
		return
	}

	if err := db.Where("1 = 1").Delete(&models.Basket{}).Error; err != nil {
		panic(err)
	}

	if user.Role == "admin" {
        http.Redirect(w, r, "/admin", http.StatusSeeOther)
        return
    }

	session, err := store.Get(r, "session-name")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session.Values["userID"] = int(user.ID)

	err = session.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Username   string
		Categories []models.Type
	}{
		Username:   user.Name,
		Categories: categories,
	}

	tmpl, err := template.ParseFiles("html/index.html")
	if err != nil {
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		return
	}

}



func runMigrations(db *gorm.DB) {
	err := db.AutoMigrate(&models.User{}, &models.Type{}, &models.Item{}, &models.Basket{}, &models.Order{})
	if err != nil {
		log.Fatal(err)
	}
}

func getCategoryAndItemData(db *gorm.DB) ([]models.Type, error) {
	var categories []models.Type
	if err := db.Preload("Items").Find(&categories).Error; err != nil {
		return nil, err
	}

	return categories, nil
}

func confirmOrder(w http.ResponseWriter, r *http.Request) {

	http.ServeFile(w, r, "html/confirm.html")
}

func admin(w http.ResponseWriter, r *http.Request) {
    dsn := "root:@tcp(127.0.0.1:3306)/canteen?charset=utf8mb4&parseTime=True&loc=Local"
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    session, err := store.Get(r, "session-name")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    userID, ok := session.Values["userID"].(int)
    if !ok {
        log.Printf("UserID not found in session or not of type int: %v", session.Values["userID"])
        http.Error(w, "User ID not found in session or not of type int", http.StatusInternalServerError)
        return
    }

    // Fetch user details
    var user models.User
    db.First(&user, userID)
	
    var orders []models.Order
    var items []models.Item
    db.Find(&orders)
    db.Find(&items)

    // Fetch orders with status "in process" and their associated items
    ordersByBasketID := make(map[uint][]models.Order)
    itemsByItemID := make(map[uint]models.Item)

    // Populate maps
    for _, order := range orders {
        ordersByBasketID[order.BasketID] = append(ordersByBasketID[order.BasketID], order)
    }
    for _, item := range items {
        itemsByItemID[item.ID] = item
    }

    // Pass data to the HTML template
    data := map[string]interface{}{
        "User":   user,
        "Orders": ordersByBasketID,
        "Items":  itemsByItemID,
    }

    tmpl, err := template.ParseFiles("html/admin.html")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    if err := tmpl.Execute(w, data); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}



func order(w http.ResponseWriter, r *http.Request) {
    dsn := "root:@tcp(127.0.0.1:3306)/canteen?charset=utf8mb4&parseTime=True&loc=Local"
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    session, err := store.Get(r, "session-name")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    userID, ok := session.Values["userID"].(int)
    if !ok {
        log.Printf("UserID not found in session or not of type int: %v", session.Values["userID"])
        http.Error(w, "User ID not found in session or not of type int", http.StatusInternalServerError)
        return
    }

	var baskets []models.Basket
	db.Where("user_id = ? AND deleted_at IS NULL", userID).Find(&baskets)

	// Create orders for items in the basket
	for _, basket := range baskets {
		order := &models.Order{
			BasketID: basket.ID,
			Status:   "In process",
		}
		db.Create(order)
	}
}


